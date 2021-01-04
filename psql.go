package stripeutil

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/andrewpillar/query"

	"github.com/stripe/stripe-go/v72"
)

// PSQL provides a way of storing Stripe resources within PostgreSQL. This will
// store the Customer, Invoice, PaymentMethod, and Subscription resource. Using
// this implementation of the Store interface would require having the
// following schema,
//
//     CREATE TABLE stripe_customers (
//         id           VARCHAR NOT NULL UNIQUE,
//         email        VARCHAR NOT NULL UNIQUE,
//         jurisdiction VARCHAR NULL,
//         created_at   TIMESTAMP NOT NULL
//     );
//
//     CREATE TABLE stripe_events (
//         id VARCHAR NOT NULL UNIQUE
//     );
//
//     CREATE TABLE stripe_invoices (
//         id                VARCHAR NOT NULL UNIQUE,
//         customer_id       VARCHAR NOT NULL,
//         number            VARCHAR NOT NULL,
//         amount            NUMERIC NOT NULL,
//         status            VARCHAR NOT NULL,
//         created_at        TIMESTAMP NOT NULL,
//         updated_at        TIMESTAMP NOT NULL
//     );
//
//     CREATE TABLE stripe_payment_methods (
//         id          VARCHAR NOT NULL UNIQUE,
//         customer_id VARCHAR NOT NULL,
//         type        VARCHAR NOT NULL,
//         info        JSON NOT NULL,
//         is_default  BOOLEAN NOT NULL DEFAULT FALSE,
//         created_at  TIMESTAMP NOT NULL
//     );
//
//     CREATE TABLE stripe_subscriptions (
//         id          VARCHAR NOT NULL UNIQUE,
//         customer_id VARCHAR NOT NULL,
//         status      VARCHAR NOT NULL,
//         started_at  TIMESTAMP NOT NULL,
//         ends_at     TIMESTAMP NULL
//     );
type PSQL struct {
	*sql.DB
}

var (
	_ Store = (*PSQL)(nil)

	customerTable      = "stripe_customers"
	eventTable         = "stripe_events"
	invoiceTable       = "stripe_invoices"
	paymentMethodTable = "stripe_payment_methods"
	subscriptionTable  = "stripe_subscriptions"
)

func getPaymentMethodInfo(pm *PaymentMethod) map[string]interface{} {
	switch pm.Type {
	case "au_becs_debit":
		return map[string]interface{}{
			"bsb_number": pm.AUBECSDebit.BSBNumber,
			"last4":      pm.AUBECSDebit.Last4,
		}
	case "bacs_debit":
		return map[string]interface{}{
			"last4":     pm.BACSDebit.Last4,
			"sort_code": pm.BACSDebit.SortCode,
		}
	case "card":
		return map[string]interface{}{
			"brand":     string(pm.Card.Brand),
			"exp_month": pm.Card.ExpMonth,
			"exp_year":  pm.Card.ExpYear,
			"last4":     pm.Card.Last4,
		}
	case "fpx":
		return map[string]interface{}{
			"bank": pm.FPX.Bank,
		}
	case "ideal":
		return map[string]interface{}{
			"bank": pm.Ideal.Bank,
			"bic":  pm.Ideal.Bic,
		}
	case "p24":
		return map[string]interface{}{
			"bank": pm.P24.Bank,
		}
	case "sepa_debit":
		return map[string]interface{}{
			"bank_code":   pm.SepaDebit.BankCode,
			"branch_code": pm.SepaDebit.BranchCode,
			"country":     pm.SepaDebit.Country,
			"last4":       pm.SepaDebit.Last4,
		}
	default:
		return nil
	}
}

func unmarshalPaymentMethodInfo(info []byte, pm *PaymentMethod) error {
	var err error

	switch pm.Type {
	case "au_becs_debit":
		err = json.Unmarshal(info, &pm.AUBECSDebit)
	case "bacs_debit":
		err = json.Unmarshal(info, &pm.BACSDebit)
	case "card":
		err = json.Unmarshal(info, &pm.Card)
	case "fpx":
		err = json.Unmarshal(info, &pm.FPX)
	case "ideal":
		err = json.Unmarshal(info, &pm.Ideal)
	case "p24":
		err = json.Unmarshal(info, &pm.P24)
	case "sepa_debit":
		err = json.Unmarshal(info, &pm.SepaDebit)
	}
	return err
}

func (p PSQL) getPaymentMethods(opts ...query.Option) ([]*PaymentMethod, error) {
	opts = append([]query.Option{
		query.From(paymentMethodTable),
	}, opts...)

	q := query.Select(query.Columns("*"), opts...)

	rows, err := p.Query(q.Build(), q.Args()...)

	if err != nil {
		return nil, err
	}

	pms := make([]*PaymentMethod, 0)

	for rows.Next() {
		pm := &PaymentMethod{
			PaymentMethod: &stripe.PaymentMethod{},
		}
		pm.Customer = &stripe.Customer{}

		var (
			info    []byte
			created time.Time
		)

		if err := rows.Scan(&pm.ID, &pm.Customer.ID, &pm.Type, &info, &pm.Default, &created); err != nil {
			if err != sql.ErrNoRows {
				return nil, err
			}
		}

		if err := unmarshalPaymentMethodInfo(info, pm); err != nil {
				return nil, err
		}
		pms = append(pms, pm)
	}
	return pms, nil
}

// LookupCustomer will lookup the Customer by the given email in the
// stripe_customers table and return them along with whether or not the
// Customer could be found.
func (p PSQL) LookupCustomer(email string) (*Customer, bool, error) {
	q := query.Select(
		query.Columns("*"),
		query.From(customerTable),
		query.Where("email", "=", query.Arg(email)),
	)

	c := &Customer{
		Customer: &stripe.Customer{},
	}

	var (
		jurisdiction sql.NullString
		created      time.Time
	)

	if err := p.QueryRow(q.Build(), q.Args()...).Scan(&c.ID, &c.Email, &jurisdiction, &created); err != nil {
		if err != sql.ErrNoRows {
			return nil, false, err
		}
		return nil, false, nil
	}

	c.Jurisdiction = jurisdiction.String
	c.Created = created.Unix()
	return c, true, nil
}

func (p PSQL) LookupInvoice(c *Customer, number string) (*Invoice, bool, error) {
	q := query.Select(
		query.Columns("*"),
		query.From(invoiceTable),
		query.Where("customer_id", "=", query.Arg(c.ID)),
		query.Where("number", "=", query.Arg(number)),
	)

	i := &Invoice{
		Invoice: &stripe.Invoice{},
	}
	i.Customer = &stripe.Customer{}

	var created time.Time

	row := p.QueryRow(q.Build(), q.Args()...)

	err := row.Scan(&i.ID, &i.Customer.ID, &i.Number, &i.AmountDue, &i.Status, &created, &i.Updated)

	if err != nil {
		if err != sql.ErrNoRows {
			return nil, false, err
		}
		return nil, false, nil
	}

	i.Created = created.Unix()
	return i, true, nil
}

func (p PSQL) LogEvent(id string) error {
	q := query.Select(
		query.Count("id"),
		query.From(eventTable),
		query.Where("id", "=", query.Arg(id)),
	)

	var count int64

	if err := p.QueryRow(q.Build(), q.Args()...).Scan(&count); err != nil {
		return err
	}

	if count > 0 {
		return ErrEventExists
	}

	q = query.Insert(eventTable, query.Columns("id"), query.Values(id))

	_, err := p.Exec(q.Build(), q.Args()...)
	return err
}

// Subscription will get the Subscription for the given Customer from the
// stripe_subscriptions table and return it along with whether or not the
// Subscription could be found.
func (p PSQL) Subscription(c *Customer) (*Subscription, bool, error) {
	q := query.Select(
		query.Columns("*"),
		query.From(subscriptionTable),
		query.Where("customer_id", "=", query.Arg(c.ID)),
		query.OrderDesc("started_at"),
	)

	sub := &Subscription{
		Subscription: &stripe.Subscription{
			Customer: &stripe.Customer{},
		},
	}

	var startedAt time.Time

	row := p.QueryRow(q.Build(), q.Args()...)

	if err := row.Scan(&sub.ID, &sub.Customer.ID, &sub.Status, &startedAt, &sub.EndsAt); err != nil {
		if err != sql.ErrNoRows {
			return nil, false, err
		}
		return nil, false, nil
	}

	sub.StartDate = startedAt.Unix()
	return sub, true, nil
}

// DefaultPaymentMethod will get the default PaymentMethod for the given
// Customer from the stripe_payment_methods table along with whether or not the
// PaymentMethod could be found.
func (p PSQL) DefaultPaymentMethod(c *Customer) (*PaymentMethod, bool, error) {
	q := query.Select(
		query.Columns("*"),
		query.From(paymentMethodTable),
		query.Where("customer_id", "=", query.Arg(c.ID)),
		query.Where("is_default", "=", query.Arg(true)),
	)

	pm := &PaymentMethod{
		PaymentMethod: &stripe.PaymentMethod{
			Customer: &stripe.Customer{},
		},
	}

	var (
		info    []byte
		created time.Time
	)

	row := p.QueryRow(q.Build(), q.Args()...)

	if err := row.Scan(&pm.ID, &pm.Customer.ID, &pm.Type, &info, &pm.Default, &created); err != nil {
		if err != sql.ErrNoRows {
			return nil, false, err
		}
		return nil, false, nil
	}

	pm.Created = created.Unix()

	if err := unmarshalPaymentMethodInfo(info, pm); err != nil {
		return nil, false, err
	}
	return pm, true, nil
}

func (p PSQL) Invoices(c *Customer) ([]*Invoice, error) {
	q := query.Select(
		query.Columns("*"),
		query.From(invoiceTable),
		query.Where("customer_id", "=", query.Arg(c.ID)),
		query.OrderDesc("created_at"),
	)

	rows, err := p.Query(q.Build(), q.Args()...)

	if err != nil {
		return nil, err
	}

	invs := make([]*Invoice, 0)

	for rows.Next() {
		var created time.Time

		inv := &Invoice{
			Invoice: &stripe.Invoice{},
		}
		inv.Customer = &stripe.Customer{}

		err := rows.Scan(
			&inv.ID,
			&inv.Customer.ID,
			&inv.Number,
			&inv.AmountDue,
			&inv.Status,
			&created,
			&inv.Updated,
		)

		if err != nil {
			return nil, err
		}

		inv.Created = created.Unix()
		invs = append(invs, inv)
	}
	return invs, nil
}

// PaymentMethods returns all of the PaymentMethods for the given Customer from
// the stripe_payment_methods table.
func (p PSQL) PaymentMethods(c *Customer) ([]*PaymentMethod, error) {
	return p.getPaymentMethods(
		query.Where("customer_id", "=", query.Arg(c.ID)),
		query.OrderDesc("created_at"),
	)
}

func (p PSQL) putCustomer(c *Customer) error {
	_, ok, err := p.LookupCustomer(c.Email)

	if err != nil {
		return err
	}

	if ok {
		q := query.Update(
			customerTable,
			query.Set("email", query.Arg(c.Email)),
			query.Set("jurisdiction", query.Arg(c.Jurisdiction)),
			query.Where("id", "=", query.Arg(c.ID)),
		)

		_, err = p.Exec(q.Build(), q.Args()...)
		return err
	}

	q := query.Insert(
		customerTable,
		query.Columns("id", "email", "jurisdiction", "created_at"),
		query.Values(c.ID, c.Email, c.Jurisdiction, time.Unix(c.Created, 0)),
	)

	_, err = p.Exec(q.Build(), q.Args()...)
	return err
}

func (p PSQL) putInvoice(i *Invoice) error {
	q := query.Select(
		query.Columns("id"),
		query.From(invoiceTable),
		query.Where("id", "=", query.Arg(i.ID)),
	)

	var id string

	if err := p.QueryRow(q.Build(), q.Args()...).Scan(&id); err != nil {
		if err != sql.ErrNoRows {
			return err
		}
	}

	if id == "" {
		created := time.Unix(i.Created, 0)

		q = query.Insert(
			invoiceTable,
			query.Columns("id", "customer_id", "number", "amount", "status", "created_at", "updated_at"),
			query.Values(i.ID, i.Customer.ID, i.Number, i.AmountDue, i.Status, created, created),
		)

		_, err := p.Exec(q.Build(), q.Args()...)
		return err
	}

	q = query.Update(
		invoiceTable,
		query.Set("status", query.Arg(i.Status)),
		query.Set("updated_at", query.Arg(time.Now())),
		query.Where("id", "=", query.Arg(i.ID)),
	)

	_, err := p.Exec(q.Build(), q.Args()...)
	return err
}

func (p PSQL) putPaymentMethod(pm *PaymentMethod) error {
	if pm.Default {
		q := query.Update(
			paymentMethodTable,
			query.Set("is_default", query.Arg(false)),
			query.Where("customer_id", "=", query.Arg(pm.Customer.ID)),
		)

		if _, err := p.Exec(q.Build(), q.Args()...); err != nil {
			return err
		}
	}

	q := query.Select(
		query.Columns("id"),
		query.From(paymentMethodTable),
		query.Where("id", "=", query.Arg(pm.ID)),
	)

	var id string

	if err := p.QueryRow(q.Build(), q.Args()...).Scan(&id); err != nil {
		if err != sql.ErrNoRows {
			return err
		}
	}

	if id == "" {
		info := getPaymentMethodInfo(pm)

		q = query.Insert(
			paymentMethodTable,
			query.Columns("id", "customer_id", "type", "info", "is_default", "created_at"),
			query.Values(pm.ID, pm.Customer.ID, pm.Type, info, pm.Default, time.Unix(pm.Created, 0)),
		)

		_, err := p.Exec(q.Build(), q.Args()...)
		return err
	}
	return nil
}

func (p PSQL) putSubscription(s *Subscription) error {
	q := query.Select(
		query.Columns("id"),
		query.From(subscriptionTable),
		query.Where("id", "=", query.Arg(s.ID)),
	)

	var id string

	if err := p.QueryRow(q.Build(), q.Args()...).Scan(&id); err != nil {
		if err != sql.ErrNoRows {
			return err
		}
	}

	if id == "" {
		q = query.Insert(
			subscriptionTable,
			query.Columns("id", "customer_id", "status", "started_at", "ends_at"),
			query.Values(s.ID, s.Customer.ID, s.Status, time.Unix(s.StartDate, 0), s.EndsAt),
		)

		_, err := p.Exec(q.Build(), q.Args()...)
		return err
	}

	q = query.Update(
		subscriptionTable,
		query.Set("status", query.Arg(s.Status)),
		query.Set("ends_at", query.Arg(s.EndsAt)),
		query.Where("id", "=", query.Arg(s.ID)),
	)

	_, err := p.Exec(q.Build(), q.Args()...)
	return err
}

// Put will put the given Resource into the PostgreSQL database. If the given
// Resource already exists then it will be updated in the respective table.
func (p PSQL) Put(r Resource) error {
	switch v := r.(type) {
	case *Customer:
		return p.putCustomer(v)
	case *Invoice:
		return p.putInvoice(v)
	case *PaymentMethod:
		return p.putPaymentMethod(v)
	case *Subscription:
		return p.putSubscription(v)
	default:
		return ErrUnknownResource
	}
}

func (p PSQL) Remove(r Resource) error {
	var id, table string

	switch v := r.(type) {
	case *Customer:
		id = v.ID
		table = customerTable
	case *Invoice:
		id = v.ID
		table = invoiceTable
	case *PaymentMethod:
		id = v.ID
		table = paymentMethodTable
	case *Subscription:
		id = v.ID
		table = subscriptionTable
	default:
		return nil
	}

	q := query.Delete(table, query.Where("id", "=", query.Arg(id)))

	_, err := p.Exec(q.Build(), q.Args()...)
	return err
}
