package stripeutil

import (
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/stripe/stripe-go/v72"
)

func newStore(t *testing.T) (PSQL, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()

	if err != nil {
		t.Fatal(err)
	}
	return PSQL{
		DB: db,
	}, mock
}

func Test_LookupCustomer(t *testing.T) {
	store, mock := newStore(t)
	defer store.DB.Close()

	tests := []struct {
		email         string
		expectedQuery string
		expectedOk    bool
		row           []driver.Value
	}{
		{
			"customer@example.com",
			"SELECT * FROM stripe_customers WHERE (email = $1)",
			true,
			[]driver.Value{"cus_123456", "customer@example.com", nil, time.Now()},
		},
		{
			"foo@example.com",
			"SELECT * FROM stripe_customers WHERE (email = $1)",
			false,
			[]driver.Value{},
		},
	}

	for i, test := range tests {
		rows := sqlmock.NewRows([]string{"id", "email", "jurisdiction", "created_at"})

		if len(test.row) > 0 {
			rows.AddRow(test.row...)
		}
		mock.ExpectQuery(regexp.QuoteMeta(test.expectedQuery)).WithArgs(test.email).WillReturnRows(rows)

		_, ok, err := store.LookupCustomer(test.email)

		if err != nil {
			t.Fatalf("tests[%d] - unexpected error: %s\n", i, err)
		}

		if ok != test.expectedOk {
			t.Errorf("tests[%d] - expected customer lookup to be ok=%v, it was not\n", i, test.expectedOk)
			continue
		}
	}
}

func Test_Subscription(t *testing.T) {
	store, mock := newStore(t)
	defer store.DB.Close()

	tests := []struct {
		c             *Customer
		expectedQuery string
		expectedOk    bool
		row           []driver.Value
	}{
		{
			&Customer{
				Customer: &stripe.Customer{
					ID: "cus_123456",
				},
			},
			"SELECT * FROM stripe_subscriptions WHERE (customer_id = $1)",
			true,
			[]driver.Value{"sub_123456", "cus_123456", "active", time.Now(), nil},
		},
		{
			&Customer{Customer: &stripe.Customer{}},
			"SELECT * FROM stripe_subscriptions WHERE (customer_id = $1)",
			false,
			[]driver.Value{},
		},
	}

	for i, test := range tests {
		rows := sqlmock.NewRows([]string{"id", "customer_id", "status", "started_at", "ends_at"})

		if len(test.row) > 0 {
			rows.AddRow(test.row...)
		}
		mock.ExpectQuery(regexp.QuoteMeta(test.expectedQuery)).WithArgs(test.c.ID).WillReturnRows(rows)

		_, ok, err := store.Subscription(test.c)

		if err != nil {
			t.Fatalf("tests[%d] - unexpected error: %s\n", i, err)
		}

		if ok != test.expectedOk {
			t.Errorf("tests[%d] - expected customer subscription to be ok=%v, it was not\n", i, test.expectedOk)
			continue
		}
	}
}

func Test_DefaultPaymentMethod(t *testing.T) {
	store, mock := newStore(t)
	defer store.DB.Close()

	tests := []struct {
		c             *Customer
		expectedQuery string
		expectedOk    bool
		row           []driver.Value
	}{
		{
			&Customer{
				Customer: &stripe.Customer{
					ID: "cus_123456",
				},
			},
			"SELECT * FROM stripe_payment_methods WHERE (customer_id = $1 AND is_default = $2)",
			true,
			[]driver.Value{
				"pm_123456",
				"cus_123456",
				"card",
				`{"brand": "visa", "last4": "4242", "exp_month": 2, "exp_year": 24}`,
				true,
				time.Now(),
			},
		},
		{
			&Customer{Customer: &stripe.Customer{}},
			"SELECT * FROM stripe_payment_methods WHERE (customer_id = $1 AND is_default = $2)",
			false,
			[]driver.Value{},
		},
	}

	for i, test := range tests {
		rows := mock.NewRows([]string{"id", "customer_id", "type", "info", "is_default", "created_at"})

		if len(test.row) > 0 {
			rows.AddRow(test.row...)
		}
		mock.ExpectQuery(regexp.QuoteMeta(test.expectedQuery)).WithArgs(test.c.ID, true).WillReturnRows(rows)

		_, ok, err := store.DefaultPaymentMethod(test.c)

		if err != nil {
			t.Fatalf("tests[%d] - unexpected error: %s\n", i, err)
		}

		if ok != test.expectedOk {
			t.Errorf("tests[%d] - expected customer default payment method to be ok=%v, it was not\n", i, test.expectedOk)
			continue
		}
	}
}
