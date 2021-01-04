package stripeutil

type TestStore struct {
	customers      map[string]*Customer
	invoices       map[string][]*Invoice
	paymentMethods map[string][]*PaymentMethod
	subscriptions  map[string]*Subscription
}

var _ Store = (*TestStore)(nil)

func newTestStore() TestStore {
	return TestStore{
		customers:      make(map[string]*Customer),
		invoices:       make(map[string][]*Invoice),
		paymentMethods: make(map[string][]*PaymentMethod),
		subscriptions:  make(map[string]*Subscription),
	}
}

func (s TestStore) LookupCustomer(email string) (*Customer, bool, error) {
	c, ok := s.customers[email]
	return c, ok, nil
}

func (s TestStore) LookupInvoice(c *Customer, number string) (*Invoice, bool, error) {
	invs := s.invoices[c.ID]

	for _, inv := range invs {
		if inv.Number == number {
			return inv, true, nil
		}
	}
	return nil, false, nil
}

func (s TestStore) Subscription(c *Customer) (*Subscription, bool, error) {
	sub, ok := s.subscriptions[c.ID]
	return sub, ok, nil
}

func (s TestStore) DefaultPaymentMethod(c *Customer) (*PaymentMethod, bool, error) {
	for _, pm := range s.paymentMethods[c.ID] {
		if pm.Default {
			return pm, true, nil
		}
	}
	return nil, false, nil
}

func (s TestStore) PaymentMethods(c *Customer) ([]*PaymentMethod, error) {
	return s.paymentMethods[c.ID], nil
}

func (s TestStore) Invoices(c *Customer) ([]*Invoice, error) {
	return s.invoices[c.ID], nil
}

func (s TestStore) Put(r Resource) error {
	switch v := r.(type) {
	case *Customer:
		s.customers[v.Email] = v
	case *Invoice:
		s.invoices[v.Customer.ID] = append(s.invoices[v.Customer.ID], v)
	case *Subscription:
		s.subscriptions[v.Customer.ID] = v
	case *PaymentMethod:
		s.paymentMethods[v.Customer.ID] = append(s.paymentMethods[v.Customer.ID], v)
	}
	return nil
}

func (s TestStore) LogEvent(_ string) error { return nil }

// Remove is no-op for now.
func (s TestStore) Remove(_ Resource) error { return nil }
