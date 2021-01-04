package stripeutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/stripe/stripe-go/v72"
)

// Taxes provides a way of storing the tax rates configured in Stripe against
// their respective jurisdiction. You would typically use this if you are
// storing your tax rates in a file on disk, and want them loaded up at start
// time of your application.
type Taxes struct {
	mu    sync.RWMutex
	ids   map[string]struct{}
	taxes map[string]stripe.TaxRate
}

var (
	taxRateEndpoint = "/v1/tax_rates"

	ErrUnknownJurisdiction = errors.New("unknown jurisdiction")
)

// LoadTaxes will load in all of the tax rate IDs from the given io.Reader. It
// is expected for each tax rate ID to be on its own separate line. Comments
// (lines prefixed with #) are ignored. The given errh function is used for
// handling any errors that arise when calling out to Stripe.
func LoadTaxes(s Stripe, r io.Reader, errh func(error)) (*Taxes, error) {
	t := &Taxes{
		mu:    sync.RWMutex{},
		ids:   make(map[string]struct{}),
		taxes: make(map[string]stripe.TaxRate),
	}

	if err := scanlines(r, t.loadTaxRate(s, errh)); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Taxes) loadTaxRate(s Stripe, errh func(error)) func(string) {
	return func(id string) {
		resp, err := s.Get(taxRateEndpoint + "/" + id)

		if err != nil {
			errh(fmt.Errorf("failed to get tax_rate %s: %s", id, err))
			return
		}

		defer resp.Body.Close()

		if !respCode2xx(resp.StatusCode) {
			err := s.Error(resp).(*stripe.Error)

			if resp.StatusCode >= 400 && resp.StatusCode <= 451 {
				errh(fmt.Errorf("failed to load tax_rate %s: %s", id, err.Msg))
				return
			}
			errh(fmt.Errorf("unexpected error when loading tax_rate %s: %s", id, err.Msg))
			return
		}

		var tr stripe.TaxRate

		if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
			errh(fmt.Errorf("failed to decode tax_rate %s: %s", id, err))
			return
		}

		t.mu.Lock()
		defer t.mu.Unlock()

		if _, ok := t.ids[tr.ID]; !ok {
			t.ids[tr.ID] = struct{}{}
			t.taxes[tr.Jurisdiction] = tr
		}
	}
}

// Get returns the tax rate for the given jurisdiction, if it exists in the
// underlying store.
func (t *Taxes) Get(jurisdiction string) (stripe.TaxRate, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	tr, ok := t.taxes[jurisdiction]

	if !ok {
		return tr, ErrUnknownJurisdiction
	}
	return tr, nil
}

// Reload loads in new tax rate IDs from the given io.Reader. This will return
// an error if there is any issue with reading from the given io.Reader. Any
// errors that occur when loading in the tax rates via Stripe will be handled
// via the given errh callback. This will only load in the new tax rates that
// are found.
func (t *Taxes) Reload(s Stripe, r io.Reader, errh func(error)) error {
	return scanlines(r, t.loadTaxRate(s, errh))
}
