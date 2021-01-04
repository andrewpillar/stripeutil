package stripeutil

import (
	"encoding/json"
	"strings"

	"github.com/stripe/stripe-go/v72"
)

// PaymentMethod is the PaymentMethod resource from Stripe. Embedded in this
// struct is the stripe.PaymentMethod struct from Stripe.
type PaymentMethod struct {
	*stripe.PaymentMethod

	Default bool // Default is whether or not this is a default PaymentMethod for the Customer.
}

var (
	_ Resource = (*PaymentMethod)(nil)

	paymentMethodEndpoint = "/v1/payment_methods"
)

func postPaymentMethod(s Stripe, uri string, params map[string]interface{}) (*PaymentMethod, error) {
	pm := &PaymentMethod{
		PaymentMethod: &stripe.PaymentMethod{},
	}

	resp, err := s.Post(uri, params)

	if err != nil {
		return pm, err
	}

	defer resp.Body.Close()

	if !respCode2xx(resp.StatusCode) {
		return pm, s.Error(resp)
	}

	err = json.NewDecoder(resp.Body).Decode(&pm.PaymentMethod)
	return pm, err
}

// RetrievePaymentMethod will get the PaymentMethod of the given ID from Stripe
// and return it.
func RetrievePaymentMethod(s Stripe, id string) (*PaymentMethod, error) {
	pm := &PaymentMethod{
		PaymentMethod: &stripe.PaymentMethod{
			ID: id,
		},
	}

	resp, err := s.Get(pm.Endpoint())

	if err != nil {
		return pm, err
	}

	defer resp.Body.Close()

	if !respCode2xx(resp.StatusCode) {
		return pm, s.Error(resp)
	}

	err = json.NewDecoder(resp.Body).Decode(&pm)
	return pm, err
}

// Update will update the current PaymentMethod in Stripe with the given Params.
func (pm *PaymentMethod) Update(s Stripe, params Params) error {
	var err error

	pm, err = postPaymentMethod(s, pm.Endpoint(), params)
	return err
}

// Attach will attach the current PaymentMethod to the given Customer.
func (pm *PaymentMethod) Attach(s Stripe, c *Customer) error {
	var err error

	pm, err = postPaymentMethod(s, pm.Endpoint("attach"), Params{"customer": c.ID})
	return err
}

// Detach will detach the current PaymentMethod from the Customer it was
// previously attached to.
func (pm *PaymentMethod) Detach(s Stripe) error {
	_, err := postPaymentMethod(s, pm.Endpoint("detach"), nil)
	return err
}

// Endpoint implements the Resource interface.
func (pm *PaymentMethod) Endpoint(uris ...string) string {
	endpoint := paymentMethodEndpoint

	if pm.ID != "" {
		endpoint += "/" + pm.ID
	}
	if len(uris) > 0 {
		endpoint += "/" + strings.Join(uris, "/")
	}
	return endpoint
}

// Load implements the Resource interface.
func (pm *PaymentMethod) Load(s Stripe) error {
	resp, err := s.Client.Get(pm.Endpoint())

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if !respCode2xx(resp.StatusCode) {
		return s.Error(resp)
	}
	return json.NewDecoder(resp.Body).Decode(&pm.PaymentMethod)
}
