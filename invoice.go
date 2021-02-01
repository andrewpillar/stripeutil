package stripeutil

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v72"
)

// Invoice is the Invoice resource from Striped. Embedded in this struct is the
// stripe.Invoice struct from Stripe.
type Invoice struct {
	*stripe.Invoice

	Updated time.Time // Updated is when the Invoice was last updated.
}

var (
	_ Resource = (*Invoice)(nil)

	invoiceEndpoint = "/v1/invoices"
)

// RetrieveUpcomingInvoice will retrieve the upcoming Invoice for the given
// Customer.
func RetrieveUpcomingInvoice(s Stripe, c *Customer) (*Invoice, error) {
	resp, err := s.Get(invoiceEndpoint + "/upcoming?customer=" + c.ID)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if !respCode2xx(resp.StatusCode) {
		return nil, s.Error(resp)
	}

	var inv Invoice

	if err := json.NewDecoder(resp.Body).Decode(&inv.Invoice); err != nil {
		return nil, err
	}
	return &inv, nil
}

// Endpoint implements the Resource interface.
func (i *Invoice) Endpoint(uris ...string) string {
	endpoint := invoiceEndpoint

	if i.ID != "" {
		endpoint += "/" + i.ID
	}
	if len(uris) > 0 {
		endpoint += "/" + strings.Join(uris, "/")
	}
	return endpoint
}

// Load implements the Resource interface.
func (i *Invoice) Load(s Stripe) error {
	resp, err := s.Client.Get(i.Endpoint())

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if !respCode2xx(resp.StatusCode) {
		return s.Error(resp)
	}
	return json.NewDecoder(resp.Body).Decode(&i.Invoice)
}
