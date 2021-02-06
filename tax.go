package stripeutil

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"runtime"
	"strings"
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
	rates map[string]*TaxRate
}

type TaxRate struct {
	*stripe.TaxRate
}

var (
	taxRateEndpoint = "/v1/tax_rates"

	// ErrUnknownJurisdiction denotes when a jurisdiction cannot be found in
	// the set of tax rates.
	ErrUnknownJurisdiction = errors.New("unknown jurisdiction")
)

func getr(br *bufio.Reader) (rune, error) {
	r, _, err := br.ReadRune()

	if err != nil {
		if err != io.EOF {
			return -1, err
		}
		return -1, nil
	}
	return r, nil
}

func ungetr(br *bufio.Reader) { br.UnreadRune() }

func skipline(br *bufio.Reader) error {
	r, err := getr(br)

	for r != '\n' {
		if err != nil {
			return err
		}
		r, err = getr(br)
	}
	return nil
}

func scanline(br *bufio.Reader) (string, error) {
	buf := make([]rune, 0)

	r, err := getr(br)

	for r != '\n' && r != -1 {
		if err != nil {
			return "", err
		}
		buf = append(buf, r)

		r, err = getr(br)
	}
	return string(buf), nil
}

// LoadTaxes will load in all of the tax rate IDs from the given io.Reader. It
// is expected for each tax rate ID to be on its own separate line. Comments
// (lines prefixed with #) are ignored. The given errh function is used for
// handling any errors that arise when calling out to Stripe.
func LoadTaxRates(r io.Reader, s *Stripe, errh func(error)) (*Taxes, error) {
	t := &Taxes{
		mu:    sync.RWMutex{},
		ids:   make(map[string]struct{}),
		rates: make(map[string]*TaxRate),
	}

	if err := t.Reload(r, s, errh); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Taxes) loadIds(r io.Reader) ([]string, error) {
	br := bufio.NewReader(r)

	ids := make([]string, 0)

	for {
redo:
		r, err := getr(br)

		if err != nil {
			return nil, err
		}

		if r == -1 {
			break
		}

		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			goto redo
		}

		if r == '#' {
			if err := skipline(br); err != nil {
				return nil, err
			}
			goto redo
		}

		ungetr(br)

		line, err := scanline(br)

		if err != nil {
			return nil, err
		}
		ids = append(ids, line)
	}
	return ids, nil
}

// Reload loads in new tax rate IDs from the given io.Reader. This will return
// an error if there is any issue with reading from the given io.Reader. Any
// errors that occur when loading in the tax rates via Stripe will be handled
// via the given errh callback. This will only load in the new tax rates that
// are found.
func (t *Taxes) Reload(r io.Reader, s *Stripe, errh func(error)) error {
	ids, err := t.loadIds(r)

	if err != nil {
		return err
	}

	sems := make(chan struct{}, runtime.GOMAXPROCS(0)+10)
	errs := make(chan error)

	rates := make([]*TaxRate, 0, len(ids))

	var wg sync.WaitGroup
	wg.Add(len(ids))

	for _, id := range ids {
		tr := &TaxRate{
			TaxRate: &stripe.TaxRate{
				ID: id,
			},
		}

		rates = append(rates, tr)

		go func(tr *TaxRate, id string) {
			sems <- struct{}{}
			defer func() {
				<-sems
				wg.Done()
			}()

			if err := tr.Load(s); err != nil {
				errs <- err
			}
		}(tr, id)
	}

	go func() {
		wg.Wait()
		close(errs)
	}()

	for e := range errs {
		errh(e)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	for _, tr := range rates {
		if _, ok := t.ids[tr.ID]; !ok {
			t.ids[tr.ID] = struct{}{}
			t.rates[tr.Jurisdiction] = tr
		}
	}
	return nil
}

// Get returns the tax rate for the given jurisdiction, if it exists in the
// underlying store.
func (t *Taxes) Get(jurisdiction string) (*TaxRate, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	tr, ok := t.rates[jurisdiction]

	if !ok {
		return nil, ErrUnknownJurisdiction
	}
	return tr, nil
}

// Endpoint implements the Resource interface.
func (tr *TaxRate) Endpoint(uris ...string) string {
	endpoint := taxRateEndpoint

	if tr.ID != "" {
		endpoint += "/" + tr.ID
	}

	if len(uris) > 0 {
		endpoint += "/"
	}
	return endpoint + strings.Join(uris, "/")
}

// Load implements the Resource interface.
func (tr *TaxRate) Load(s *Stripe) error {
	resp, err := s.Get(tr.Endpoint())

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if !respCode2xx(resp.StatusCode) {
		return s.Error(resp)
	}
	return json.NewDecoder(resp.Body).Decode(tr)
}
