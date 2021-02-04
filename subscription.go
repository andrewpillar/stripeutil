package stripeutil

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v72"
)

// Subscription is the Subscription resource from Stripe. Embedded in this
// struct is the stripe.Subscription struct from Stripe.
type Subscription struct {
	*stripe.Subscription

	EndsAt sql.NullTime // EndsAt is the time the Subscription ends if it was cancelled.
}

var (
	_ Resource = (*Subscription)(nil)

	subscriptionEndpoint = "/v1/subscriptions"

	validSubscriptionStatuses = map[stripe.SubscriptionStatus]struct{}{
		stripe.SubscriptionStatusAll:      {},
		stripe.SubscriptionStatusActive:   {},
		stripe.SubscriptionStatusTrialing: {},
	}
)

func postSubscription(st Stripe, uri string, params map[string]interface{}) (*Subscription, error) {
	sub := &Subscription{}

	resp, err := st.Post(uri, params)

	if err != nil {
		return sub, err
	}

	defer resp.Body.Close()

	if !respCode2xx(resp.StatusCode) {
		return sub, st.Error(resp)
	}

	err = json.NewDecoder(resp.Body).Decode(&sub.Subscription)
	return sub, err
}

// CreateSubscription will create a new Subscription in Stripe with the given
// request Params.
func CreateSubscription(st Stripe, params Params) (*Subscription, error) {
	return postSubscription(st, subscriptionEndpoint, params)
}

// Reactivate will reactivate the current subscription by setting the property
// cancel_at_period_end to false. This will set the EndsAt field to be invalid.
func (s *Subscription) Reactivate(st Stripe) error {
	if err := s.Update(st, Params{"cancel_at_period_end": false}); err != nil {
		return err
	}
	s.EndsAt = sql.NullTime{}
	return nil
}

// Cancel will cancel the current Subscription at the end of the Subscription
// Period. This will set the EndsAt field to the CurrentPeriodEnd of the
// Subscription.
func (s *Subscription) Cancel(st Stripe) error {
	if err :=  s.Update(st, Params{"cancel_at_period_end": true}); err != nil {
		return err
	}
	s.EndsAt = sql.NullTime{
		Time:  time.Unix(s.CurrentPeriodEnd, 0),
		Valid: true,
	}
	return nil
}

// Update will update the current Subscription in Stripe with the given Params.
func (s *Subscription) Update(st Stripe, params Params) error {
	s1, err := postSubscription(st, s.Endpoint(), params)

	if err != nil {
		return err
	}

	(*s) = (*s1)
	return nil
}

// Endpoint implements the Resource interface.
func (s *Subscription) Endpoint(uris ...string) string {
	endpoint := subscriptionEndpoint

	if s.ID != "" {
		endpoint += "/" + s.ID
	}
	if len(uris) > 0 {
		endpoint += "/" + strings.Join(uris, "/")
	}
	return endpoint
}

// WithinGrace will return true if the current Subscription has been canceled
// but stil lies within the grace period. If the Subscription has not been
// canceled then this will always return true.
func (s *Subscription) WithinGrace() bool {
	if s == nil {
		return false
	}

	if !s.EndsAt.Valid {
		return false
	}
	return time.Now().Before(s.EndsAt.Time)
}

// Valid will return whether or not the current Subscription is valid. A
// Subscription is considered valid if the status is one of, "all", "active",
// or "trialing", or if the Subscription was cancelled but the current time
// is before the EndsAt date.
func (s *Subscription) Valid() bool {
	if s == nil {
		return false
	}

	if s.EndsAt.Valid {
		return time.Now().Before(s.EndsAt.Time)
	}

	if _, ok := validSubscriptionStatuses[s.Status]; ok {
		return ok
	}
	return false
}

// Load implements the Resource interface.
func (s *Subscription) Load(st Stripe) error {
	resp, err := st.Client.Get(s.Endpoint())

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if !respCode2xx(resp.StatusCode) {
		return st.Error(resp)
	}
	return json.NewDecoder(resp.Body).Decode(&s)
}
