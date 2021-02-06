package stripeutil

import (
	"encoding/json"
	"strings"

	"github.com/stripe/stripe-go/v72"
)

// Customer is the Customer resource from Stripe. Embedded in this struct is
// the stripe.Customer struct from Stripe.
type Customer struct {
	*stripe.Customer

	Jurisdiction string
}

var (
	_ Resource = (*Customer)(nil)

	customerEndpoint = "/v1/customers"
)

func postCustomer(s *Stripe, uri string, params Params) (*Customer, error) {
	c := &Customer{}

	resp, err := s.Post(uri, params)

	if err != nil {
		return c, err
	}

	defer resp.Body.Close()

	if !respCode2xx(resp.StatusCode) {
		return c, s.Error(resp)
	}

	err = json.NewDecoder(resp.Body).Decode(&c.Customer)
	return c, err
}

// CreateCustomer creates a new Customer in Stripe with the given Params and
// returns it.
func CreateCustomer(s *Stripe, params Params) (*Customer, error) {
	return postCustomer(s, customerEndpoint, params)
}

// Endpoint implements the Resource interface.
func (c *Customer) Endpoint(uris ...string) string {
	endpoint := customerEndpoint

	if c.ID != "" {
		endpoint += "/" + c.ID
	}

	if len(uris) > 0 {
		endpoint += "/"
	}
	return endpoint + strings.Join(uris, "/")
}

// Load implements the Resource interface.
func (c *Customer) Load(s *Stripe) error {
	resp, err := s.Client.Get(c.Endpoint())

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if !respCode2xx(resp.StatusCode) {
		return s.Error(resp)
	}
	return json.NewDecoder(resp.Body).Decode(&c.Customer)
}

// Update will update the current Customer in Stripe with the given Params.
func (c *Customer) Update(s *Stripe, params Params) error {
	c1, err := postCustomer(s, c.Endpoint(), params)

	if err != nil {
		return err
	}
	(*c) = (*c1)
	return nil
}
