package stripeutil

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	stripelib "github.com/stripe/stripe-go/v72"
)

func Test_Params(t *testing.T) {
	tests := []struct {
		params   Params
		expected string
	}{
		{
			Params{"email": "me@example.com"},
			"email=me%40example.com",
		},
		{
			Params{
				"invoice_settings": Params{
					"default_payment_method": "pm_123456",
				},
			},
			"invoice_settings[default_payment_method]=pm_123456",
		},
		{
			Params{
				"customer": "cu_123456",
				"items": []Params{
					{"price": "pr_123456"},
				},
				"expand": []string{"latest_invoice.payment_intent"},
			},
			"customer=cu_123456&expand[0]=latest_invoice.payment_intent&items[0][price]=pr_123456",
		},
		{
			Params{
				"amount":               2000,
				"currency":             "gbp",
				"payment_method_types": []string{"card"},
			},
			"amount=2000&currency=gbp&payment_method_types[0]=card",
		},
	}

	for i, test := range tests {
		encoded := test.params.Encode()

		if encoded != test.expected {
			t.Errorf("tests[%d] - unexpected encoding, expected=%q, got=%q\n", i, test.expected, encoded)
		}
	}
}

func Test_Stripe(t *testing.T) {
	secret := os.Getenv("STRIPE_SECRET")
	price := os.Getenv("STRIPE_PRICE")

	if secret == "" || price == "" {
		t.Skip("STRIPE_SECRET and STRIPE_PRICE not set, skipping")
	}

	store := newTestStore()
	stripe := New(secret, store)

	c, err := stripe.Customer("customer@stripeutil.test")

	if err != nil {
		t.Fatal(err)
	}

	resp, err := stripe.Post(paymentMethodEndpoint, Params{
		"card": Params{
			"number":    "4242424242424242",
			"exp_month": "12",
			"exp_year":  strconv.FormatInt(int64(time.Now().Add(time.Hour*24*365).Year()), 10),
			"cvc":       "111",
		},
		"type": "card",
	})

	if err != nil {
		t.Fatal(err)
	}

	defer resp.Body.Close()

	if !respCode2xx(resp.StatusCode) {
		t.Fatal(stripe.Error(resp))
	}

	pm := &PaymentMethod{
		PaymentMethod: &stripelib.PaymentMethod{},
	}

	json.NewDecoder(resp.Body).Decode(pm)

	if err := pm.Attach(stripe, c); err != nil {
		t.Fatal(err)
	}

	_, err = stripe.Subscribe(c, pm, Params{
		"items": []Params{
			{"price": price},
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	if _, err := stripe.Unsubscribe(c); err != nil {
		t.Fatal(err)
	}

	resp1, err := stripe.Delete(c.Endpoint())

	if err != nil {
		t.Fatal(err)
	}

	defer resp1.Body.Close()

	if !respCode2xx(resp1.StatusCode) {
		t.Fatal(stripe.Error(resp1))
	}
}
