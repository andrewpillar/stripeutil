package stripeutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/stripe/stripe-go/v72"
)

// Client is a simple HTTP client for the Stripe API. This can be configured to
// use specific version of the Stripe API. Each request made via this client
// will be automatically configured to talk to the Stripe API with the
// necessary headers.
type Client struct {
	http.Client

	secret   string
	endpoint string
	version  string
}

type Error struct {
	Status string `json:"-"`
	Err    struct {
		Message string
		Type    string
	} `json:"error"`
}

// ErrPaymentIntent represents a PaymentIntent with an invalid status. This
// will contain the ID of the original PaymentIntent, and the status that
// caused the error in the first place.
type ErrPaymentIntent struct {
	ID     string
	Status stripe.PaymentIntentStatus
}

// Resource represents a resource that has been retrieved by Stripe.
type Resource interface {
	// Endpoint will return the URI for the current Resource from the Stripe
	// API. The given uris will be appended to the final endpoint. If the
	// Resource does not have an ID set on it, then the base endpoint for the
	// Resource should be returned.
	Endpoint(uris ...string) string

	// Load will use the given Stripe client to load in the resource from the
	// Stripe API using the Resource's endpoint. This should overwrite the
	// fields in the Resource with the decoded response from Stripe.
	Load(Stripe) error
}

// Store provides an interface for storing and retrieving resources that have
// been received by the Stripe API in an underlying data store such as a
// database.
type Store interface {
	// LookupCustomer will lookup the customer by the given email from within
	// the underlying data store. Whether or not the customer could be found
	// is denoted by the returned bool value.
	LookupCustomer(email string) (*Customer, bool, error)

	// LookupInvoice will lookup the invoice for the given customer by the
	// given invoice number. Whether or not the invoice could be found is
	// denoted by the returned bool value.
	LookupInvoice(c *Customer, number string) (*Invoice, bool, error)

	// LogEvent will store the given event ID in the underlying store. If the
	// given event ID already exists, then this should return ErrEventExists.
	LogEvent(string) error

	// Subscription returns the subscription for the given Customer. Whether or
	// not the Customer has a subscription will be denoted by the returned bool
	// value.
	Subscription(*Customer) (*Subscription, bool, error)

	// DefaultPaymentMethod returns the default payment method for the given
	// Customer. Whether or not the Customer has a default payment method is
	// denoted by the returned bool value.
	DefaultPaymentMethod(*Customer) (*PaymentMethod, bool, error)

	// Invoices returns all of the invoices for the given customer. The returned
	// invoices should be sorted from newest to oldest.
	Invoices(*Customer) ([]*Invoice, error)

	// PaymentMethods returns all of the payment methods that has been attached
	// to the given Customer.
	PaymentMethods(*Customer) ([]*PaymentMethod, error)

	// Put will put the given Resource into the underlying data store. If the
	// given Resource already exists in the data store, then that should simply
	// be updated. If the given Resource is the PaymentMethod resource, then a
	// check should be done to ensure that only one PaymentMethod for a Customer
	// is the default PaymentMethod.
	Put(Resource) error

	// Remove will remove the given Resource from the underlying data store. If
	// the given Resource cannot be found then this returns nil.
	Remove(Resource) error
}

// Stripe provides a simple way of managing the flow of creating customers and
// subscriptions, and for storing them in a data store.
type Stripe struct {
	Client
	Store
}

type pair struct {
	key   string
	value interface{}
}

// Params is used for defining the parameters that are passed in the body of a
// Request made to the Stripe API. This will be encoded into a valid
// x-www-form-urlencoded payload.
type Params map[string]interface{}

var (
	ErrEventExists     = errors.New("event exists")
	ErrUnknownResource = errors.New("unknown resource")
)

// encodeSliceToPairs will encode an arbitrary slice of values into a slice of
// pairs. It is expected for the given reflect.Value to be a of reflect.Slice.
// The given key denotes the key in the original parameter set for which the
// slice belongs to. Each pair encoded will have a key of key[i] where key is
// the passed key argument, and i is of the pair's value in the slice.
func encodeSliceToPairs(key string, val reflect.Value) []pair {
	pairs := make([]pair, 0)

	for i := 0; i < val.Len(); i++ {
		k := key + "[" + strconv.FormatInt(int64(i), 10) + "]"
		v := val.Index(i).Interface()

		if p, ok := v.(Params); ok {
			pairs = append(pairs, p.encodeToPairs(k)...)
			continue
		}
		pairs = append(pairs, pair{
			key:   k,
			value: v,
		})
	}
	return pairs
}

func respCode2xx(code int) bool { return code >= 200 && code < 300 }

// New configures a new Stripe client with the given secret for authenticatio
// and Store for storing/retrieving resources.
func New(secret string, s Store) Stripe {
	return Stripe{
		Store:  s,
		Client: NewClient(stripe.APIVersion, secret),
	}
}

// NewClient configures a new Client for interfacing with the Stripe API using
// the given version, and secret for authentication.
func NewClient(version, secret string) Client {
	return Client{
		secret:   secret,
		endpoint: stripe.APIURL,
		version:  version,
	}
}

func (e *Error) Error() string {
	return fmt.Errorf("stripeutil/stripe.go: stripe api error %s: %s", e.Status, e.Err.Message)
}

func (e ErrPaymentIntent) Error() string { return string(e.Status) }

func (p pair) encode() string { return p.key + "=" + url.QueryEscape(fmt.Sprintf("%v", p.value)) }

func (p Params) encodeToPairs(parent string) []pair {
	pairs := make([]pair, 0)

	for k, v := range p {
		if parent != "" {
			k = parent + "[" + k + "]"
		}

		if p1, ok := v.(Params); ok {
			pairs = append(pairs, p1.encodeToPairs(k)...)
			continue
		}

		if reflect.TypeOf(v).Kind() == reflect.Slice {
			pairs = append(pairs, encodeSliceToPairs(k, reflect.ValueOf(v))...)
			continue
		}
		pairs = append(pairs, pair{
			key:   k,
			value: v,
		})
	}
	return pairs
}

// Encode encodes the current Params into an x-www-form-urlencoded string and
// returns it.
func (p Params) Encode() string {
	pairs := make([]string, 0)

	for _, pair := range p.encodeToPairs("") {
		pairs = append(pairs, pair.encode())
	}

	sort.Strings(pairs)
	return strings.Join(pairs, "&")
}

// Reader returns an io.Reader for the x-www-form-urlencoded string of the
// current Params.
func (p Params) Reader() io.Reader { return strings.NewReader(p.Encode()) }

func (c Client) do(method, uri string, r io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, c.endpoint + "/" + uri, r)

	if err != nil {
		return nil, err
	}

	contentType := map[string]string{
		"POST":   "application/x-www-form-urlencoded",
		"GET":    "application/json; charset=utf-8",
		"DELETE": "application/json; charset=utf-8",
	}

	req.Header.Set("Authorization", "Bearer " + c.secret)
	req.Header.Set("Content-Type", contentType[method])
	req.Header.Set("Stripe-Version", c.version)

	return c.Do(req)
}

// Error decodes an error from the Stripe API from the given http.Response and
// returns it as a pointer to Error.
func (c Client) Error(resp *http.Response) error {
	e := &Error{
		Status: resp.Status,
	}

	if err := json.NewDecoder(resp.Body).Decode(e); err != nil {
		return err
	}
	return e
}

// Get will send a GET request to the given URI of the Stripe API.
func (c Client) Get(uri string) (*http.Response, error) {
	return c.do("GET", uri, nil)
}

// Post will send a POST request to the given URI of the Stripe API, along with
// the given io.Reader as the request body.
func (c Client) Post(uri string, r io.Reader) (*http.Response, error) {
	return c.do("POST", uri, r)
}

// Delete will send a DELETE request to the given URI of the Stripe API.
func (c Client) Delete(uri string) (*http.Response, error) {
	return c.do("DELETE", uri, nil)
}

// Post will send a POST request to the given URI of the Stripe API.
func (s Stripe) Post(uri string, params Params) (*http.Response, error) {
	return s.Client.Post(uri, params.Reader())
}

// Customer will get the Stripe customer by the given email. If a customer does
// not exist in the underlying data store then one is created via Stripe and
// subsequently stored in the underlying data store.
func (s Stripe) Customer(email string) (*Customer, error) {
	c, ok, err := s.Store.LookupCustomer(email)

	if err != nil {
		return c, err
	}

	if !ok {
		resp, err := s.Post(customerEndpoint, Params{"email": email})

		if err != nil {
			return c, err
		}

		defer resp.Body.Close()

		if !respCode2xx(resp.StatusCode) {
			return c, s.Error(resp)
		}

		c = &Customer{
			Customer: &stripe.Customer{},
		}

		if err := json.NewDecoder(resp.Body).Decode(&c.Customer); err != nil {
			return c, err
		}

		if err := s.Store.Put(c); err != nil {
			return c, err
		}
	}
	return c, err
}

// Subscribe creates a new subscription for the given Customer using the given
// PaymentMethod. The given Params will be passed through directly to the
// request that creates the Subscription in Stripe. The given PaymentMethod and
// returned Subscription will be stored in the underlying data store. If the
// payment for the Subscription fails then this will be returned via
// ErrPaymentIntent.
func (s Stripe) Subscribe(c *Customer, pm *PaymentMethod, params Params) (*Subscription, error) {
	sub, ok, err := s.Subscription(c)

	if err != nil {
		return sub, err
	}

	if err := pm.Attach(s, c); err != nil {
		return sub, err
	}

	err = c.Update(s, Params{
		"invoice_settings": Params{
			"default_payment_method": pm.ID,
		},
	})

	if err != nil {
		return sub, err
	}

	pm.Customer = c.Customer
	pm.Default = true

	if err := s.Store.Put(pm); err != nil {
		return sub, err
	}

	if ok {
		if sub.Valid() {
			return sub, nil
		}
	}

	params["customer"] = c.ID
	params["expand"] = []string{"latest_invoice.payment_intent"}

	sub, err = CreateSubscription(s, params)

	if err != nil {
		return sub, err
	}

	statuses := map[stripe.PaymentIntentStatus]struct{}{
		stripe.PaymentIntentStatusProcessing: {},
		stripe.PaymentIntentStatusSucceeded:  {},
	}

	if _, ok := statuses[sub.LatestInvoice.PaymentIntent.Status]; ok {
		if err := s.Store.Put(sub); err != nil {
			return sub, err
		}

		err = s.Store.Put(&Invoice{
			Invoice: sub.LatestInvoice,
		})

		return sub, err
	}
	return sub, ErrPaymentIntent{
		ID:     sub.LatestInvoice.ID,
		Status: sub.LatestInvoice.PaymentIntent.Status,
	}
}

// Resubscribe will reactivate the given Customer's Subscription, if that
// Subscription was canceled and lies within the grace period.
func (s Stripe) Resubscribe(c *Customer) error {
	sub, ok, err := s.Subscription(c)

	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	if !sub.EndsAt.Valid {
		return nil
	}

	if err := sub.Reactivate(s); err != nil {
		return err
	}
	return s.Put(sub)
}

// Unsubscribe will cancel the subscription for the given Customer if that
// subscription exists, and is valid. This will cancel the subscription at the
// period end for the customer, and update it in the underlying store.
func (s Stripe) Unsubscribe(c *Customer) (*Subscription, error) {
	sub, ok, err := s.Subscription(c)

	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, nil
	}

	if !sub.Valid() {
		return nil, nil
	}

	if sub.EndsAt.Valid {
		return sub, nil
	}

	if err := sub.Cancel(s); err != nil {
		return nil, err
	}

	if err := s.Put(sub); err != nil {
		return nil, err
	}
	return sub, nil
}
