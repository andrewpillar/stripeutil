package stripeutil

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	stripelib "github.com/stripe/stripe-go/v72"
)

func Test_TaxRate(t *testing.T) {
	secret := os.Getenv("STRIPE_SECRET")

	if secret == "" {
		t.Skip("STRIPE_SECRET not set, skipping")
	}

	store := newTestStore()
	stripe := New(secret, store)

	resp, err := stripe.Post(taxRateEndpoint, Params{
		"display_name": "VAT",
		"inclusive":    false,
		"percentage":   20,
		"jurisdiction": "uk",
	})

	if err != nil {
		t.Fatal(err)
	}

	defer resp.Body.Close()

	if !respCode2xx(resp.StatusCode) {
		t.Fatal(stripe.Error(resp))
	}

	tr := &TaxRate{
		TaxRate: &stripelib.TaxRate{},
	}

	json.NewDecoder(resp.Body).Decode(tr)

	buf := bytes.NewBufferString(`

# This is an example text file containing the tax rate IDs we want to load in.
`)
	buf.WriteString(tr.ID)

	rates, err := LoadTaxRates(buf, stripe, func(err error) {
		t.Errorf("failed to load tax rate: %s\n", err)
	})

	if err != nil {
		t.Fatal(err)
	}

	tr, err = rates.Get("uk")

	if err != nil {
		t.Fatal(err)
	}

	resp1, err := stripe.Post(tr.Endpoint(), Params{
		"active": false,
	})

	if err != nil {
		t.Fatal(err)
	}

	defer resp1.Body.Close()

	if !respCode2xx(resp.StatusCode) {
		t.Fatal(stripe.Error(resp1))
	}
}
