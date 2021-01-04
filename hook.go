package stripeutil

import (
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/webhook"
)

// HookHandlerFunc is the handler function that is registered agains an event.
// This is like an http.HandlerFunc, only the first argument it is passed is
// the decoded event sent from stripe.
type HookHandlerFunc func(stripe.Event, http.ResponseWriter, *http.Request)

// HookHandler provides a way of registering handlers against the different
// events emitted by Stripe.
type HookHandler struct {
	mu     sync.RWMutex
	errh   func(error)
	secret string
	store  Store
	events map[string]HookHandlerFunc
}

// NewHookHandler returns a HookHandler using the given secret for request
// verification, and the given callback for handling any errors that occur
// during request verification.
func NewHookHandler(secret string, s Store, errh func(error)) *HookHandler {
	return &HookHandler{
		mu:     sync.RWMutex{},
		errh:   errh,
		secret: secret,
		store:  s,
		events: make(map[string]HookHandlerFunc),
	}
}

// Handler registers a new handler for the given event. If a handler was
// already registered against the given event, then that handler will be
// overwritten with the new handler.
func (h *HookHandler) Handle(event string, fn HookHandlerFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events[event] = fn
}

// HandlerFunc should be registered in the route multiplexer being used to
// register routes in the web server. For example,
//
//     mux := http.NewServeMux()
//     mux.HandleFunc("/stripe-hook", hook.HandlerFunc)
//
// this would cause the HookHandler to handle all of the requests sent to the
// "/stripe-hook" endpoint.
func (h *HookHandler) HandlerFunc(w http.ResponseWriter, r *http.Request) {
	payload, err := ioutil.ReadAll(r.Body)

	if err != nil {
		h.errh(err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), h.secret)

	if err != nil {
		h.errh(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if h.store != nil {
		if err := h.store.LogEvent(event.ID); err != nil {
			if err != ErrEventExists {
				h.errh(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusAccepted)
			return
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if fn, ok := h.events[event.Type]; ok {
		fn(event, w, r)
		return
	}
	w.WriteHeader(http.StatusOK)
}
