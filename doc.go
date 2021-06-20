// package stripeutil provides some utility functions and data structures for
// working with the Stripe API for builing a SaaS application. This provides
// simple ways of creating Customers and Subscriptions, and storing them in a
// user defined data store along with their PaymentMethods and Invoices. This
// also provides a simple way of handling webhook events that are emitted from
// Stripe.
//
// stripeutil.Stripe is the main way to interact with the Stripe API. This is
// supposed to be used along with the stripeutil.Store interface which allows
// for storing the resources retrieved from Stripe. Below is a brief example as
// to how this library would be used to implement a subscription flow,
//
//     stripe := stripeutil.New(os.Getenv("STRIPE_SECRET"), store)
//
//     c, err := stripe.Customer("me@example.com")
//
//     if err != nil {
//         panic(err) // Don't actually do this.
//     }
//
//     // Assume the payment method ID has been passed through the a client, as
//     // opposed to being hardcoded.
//     pm, err := stripeutil.RetrievePaymentMethod(stripe, "pm_123456")
//
//     if err != nil {
//         panic(err) // Handle error properly.
//     }
//
//     // Create a subscription for the given customer with the given payment
//     // method.
//     sub, err := stripe.Subscribe(c, pm, Params{
//         "items": []Params{
//             {"price": "price_123456"},
//         },
//     })
//
//     if err != nil {
//         panic(err) // Be more graceful when you do this.
//     }
//
// the above code will first lookup the customer via the given stripeutil.Store
// implementation we pass. If a customer cannot be found then one is created in
// Stripe with the given email address and subsequently stored, before being
// returned. After this, we then retrieve the given payment method from Stripe,
// and pass this, along with the customer to the Subscribe call. We also
// specify the request parameters we wish to have set when creating a
// subscription in Stripe. Under the hood, stripeutil.Stripe will do the
// following when Subscribe is called,
//
// - Retrieve the subscription for the given customer from the underlying store
//
// - Attach the given payment method to the given customer
//
// - Update the customer's default payment method to what was given
//
// - Store the given payment method in the underlying store
//
// - Return the subscription for the given customer, if one was found otherwise...
//
// - ...a new subscription is created for the customer, and returned if the
// invoice status is valid
//
// And below is how a cancellation flow of a subscription would work with this
// library,
//
//     c, err := stripe.Customer("me@example.com")
//
//     if err != nil {
//         panic(err) // If a recover is used then a panic is fine right?
//     }
//
//     if err := stripe.Unsubscribe(c); err != nil {
//         panic(err) // Let's hope a recover is somewhere...
//     }
//
// with the above, we lookup the customer similar to how we did before, and pass
// them to the Unsubscribe call. This will update the customer's subscription
// to cancel at the end period, and update the subscription in the underlying
// store. However, if the customer's subscription cannot be found in the
// underlying store, or is not valid then nothing happens and nil is returned.
//
// stripeutil.Store is an interface that allows for storing the resources
// retrieved from Stripe. An implementation of this interface for PostgreSQL
// comes with this library out of the box. stripeutil.Stripe, depends on this
// interface for storing the customer, invoice, and subscription invoices during
// the Subscribe flow.
//
// stripeutil.Stripe is what is primarily used for interfacing with the Stripe
// API. This depends on the stripeutil.Store interface, as previously mentioned,
// for storing the resources retrieved from Stripe.
//
// stripeutil.Params allows for specifying the request parameters to set in the
// body of the request sent to Stripe. This is encoded to x-www-url-formencoded,
// when sent in a request, for example,
//
//     stripeutil.Params{
//         "invoice_settings": stripeutil.Params{
//             "default_payment_method": "pm_123456",
//         },
//     }
//
// would be encoded to,
//
//     invoice_settings[default_payment_method]=pm_123456
//
// stripeutil.Stripe has a Post method that accepts a stripeutil.Params
// argument, this can be used for making more explicit calls to Stripe,
//
//     resp, err := stripe.Post("/v1/payment_intents", stripeutil.Params{
//         "amount":               2000,
//         "currency":             "gbp",
//         "payment_method_types": []string{"card"},
//     })
//
// the returned *http.Response can be used as usual.
//
// stripeutil.Client is a thin HTTP client for the Stripe API. All HTTP requests
// made through this client will be configured to talk to Stripe. This is
// embedded inside of stripeutil.Stripe so you can do stuff like,
//
//     resp, err := stripe.Get("/v1/customers")
//
// for example, to get the customers you have. A new client can be created via
// stripeutil.NewClient, and through this you can configured which version of
// the Stripe API to use,
//
//     client := stripeutil.NewClient("2006-01-02", os.Getenv("STRIPE_SECRET"))
//
// If using an older/newer version of the Stripe API this way then it is highly
// recommended that you do not use stripeutil.Stripe and instead perform all
// interactions via stripeutil.Client. This is because stripeutil.Stripe relies
// on the stripe/stripe-go SDK, so the versions may not match if you do this.
package stripeutil
