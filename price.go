package stripeutil

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/stripe/stripe-go/v72"
)

// Prices provides a way of storing the prices and their respective product
// configured in Stripe. You would typically use this if you are storing your
// prices in a file on disk, and want them loaded up at start time of your
// application.
type Prices struct {
	mu     sync.RWMutex
	ids    map[string]struct{}
	prices []Price
}

type Price struct {
	*stripe.Price
}

var (
	priceEndpoint   = "/v1/prices"
	productEndpoint = "/v1/products"
)

// LoadPrices will load in all of the price IDs from the given io.Reader. It is
// expected for each price ID to be on its own separate line. Comments (lines
// prefixed with #) are ignored. The given errh function is used for handling
// any errors that arise when calling out to Stripe.
func LoadPrices(s Stripe, r io.Reader, errh func(error)) (Prices, error) {
	p := Prices{
		mu:     sync.RWMutex{},
		ids:    make(map[string]struct{}),
		prices: make([]Price, 0),
	}

	if err := scanlines(r, p.loadPrice(s, errh)); err != nil {
		return p, err
	}
	return p, nil
}

func (p *Prices) loadPrice(s Stripe, errh func(error)) func(string) {
	return func(id string) {
		resp, err := s.Get(priceEndpoint + "/" + id)

		if err != nil {
			errh(fmt.Errorf("failed to get price %s: %s", id, err))
			return
		}

		defer resp.Body.Close()

		if !respCode2xx(resp.StatusCode) {
			err := s.Error(resp).(*stripe.Error)

			if resp.StatusCode >= 400 && resp.StatusCode <= 451 {
				errh(fmt.Errorf("failed to load price %s: %s", id, err.Msg))
				return
			}
			errh(fmt.Errorf("unexpected error when loading price %s: %s", id, err.Msg))
			return
		}

		var pr stripe.Price

		if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
			errh(fmt.Errorf("failed to decode price %s: %s", id, err))
			return
		}

		resp1, err := s.Get(productEndpoint + "/" + pr.Product.ID)

		if err != nil {
			errh(fmt.Errorf("failed to get price %s: %s", id, err))
			return
		}

		if !respCode2xx(resp1.StatusCode) {
			err := s.Error(resp1).(*stripe.Error)

			if resp.StatusCode >= 400 && resp.StatusCode <= 451 {
				errh(fmt.Errorf("failed to load product %s: %s", pr.Product.ID, err.Msg))
				return
			}
			errh(fmt.Errorf("unexpected error when loading product %s: %s", pr.Product.ID, err.Msg))
			return
		}

		if err := json.NewDecoder(resp1.Body).Decode(&pr.Product); err != nil {
			errh(fmt.Errorf("failed to decode product %s: %s", id, err))
			return
		}

		p.mu.Lock()
		defer p.mu.Unlock()

		if _, ok := p.ids[pr.ID]; !ok {
			p.ids[pr.ID] = struct{}{}
			p.prices = append(p.prices, Price{
				Price: &pr,
			})
		}
	}
}

// Reload loads in new price IDs from the given io.Reader. This will return an
// error if there is any issue with reading from the given io.Reader. Any
// errors that occur when loading in the prices via Stripe will be handled via
// the given errh callback. This will only load in the new prices that are
// found.
func (p *Prices) Reload(s Stripe, r io.Reader, errh func(error)) error {
	return scanlines(r, p.loadPrice(s, errh))
}

// Slice returns the slice of prices.
func (p *Prices) Slice() []Price { return p.prices }
