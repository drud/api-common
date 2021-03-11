package state

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/stripe/stripe-go"
	"k8s.io/klog"
)

const maxBodyBytes = int64(65536)

func (s *Manager) handler(w http.ResponseWriter, req *http.Request) {
	req.Body = http.MaxBytesReader(w, req.Body, maxBodyBytes)
	payload, err := ioutil.ReadAll(req.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading request body: %v\n", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	event := stripe.Event{}
	if err := json.Unmarshal(payload, &event); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse webhook body json: %v\n", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Unmarshal the event data into an appropriate struct depending on its Type
	switch event.Type {
	//
	// Create/Update events
	//
	case "customer.subscription.created", "customer.subscription.updated":
		var stripeType stripe.Subscription
		err = s.CreateOrUpdate(req.Context(), event.Data.Raw, &stripeType)
		if err != nil {
			klog.Infof("error CreateOrUpdate %T: %v", stripeType, err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	case "customer.created", "customer.updated":
		fmt.Fprintf(os.Stdout, "handled event type: %s\n", event.Type)
		var stripeType stripe.Customer
		err = s.CreateOrUpdate(req.Context(), event.Data.Raw, &stripeType)
		if err != nil {
			klog.Infof("error CreateOrUpdate %T: %v", stripeType, err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	case "plan.created", "plan.updated":
		fmt.Fprintf(os.Stdout, "handled event type: %s\n", event.Type)
		var stripeType stripe.Plan
		err = s.CreateOrUpdate(req.Context(), event.Data.Raw, &stripeType)
		if err != nil {
			klog.Infof("error CreateOrUpdate %T: %v", stripeType, err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	case "product.created", "product.updated":
		fmt.Fprintf(os.Stdout, "handled event type: %s\n", event.Type)
		var stripeType stripe.Product
		err = s.CreateOrUpdate(req.Context(), event.Data.Raw, &stripeType)
		if err != nil {
			klog.Infof("error CreateOrUpdate %T: %v", stripeType, err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	//
	// Delete events
	//
	case "customer.subscription.deleted":
		fmt.Fprintf(os.Stdout, "handled event type: %s\n", event.Type)
		var stripeType stripe.Subscription
		err = s.Delete(req.Context(), event.Data.Raw, &stripeType)
		if err != nil {
			klog.Infof("error Delete %T: %v", stripeType, err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	case "customer.deleted":
		fmt.Fprintf(os.Stdout, "handled event type: %s\n", event.Type)
		var stripeType stripe.Customer
		err = s.Delete(req.Context(), event.Data.Raw, &stripeType)
		if err != nil {
			klog.Infof("error Delete %T: %v", stripeType, err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	case "plan.deleted":
		fmt.Fprintf(os.Stdout, "handled event type: %s\n", event.Type)
		var stripeType stripe.Plan
		err = s.Delete(req.Context(), event.Data.Raw, &stripeType)
		if err != nil {
			klog.Infof("error Delete %T: %v", stripeType, err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	case "product.deleted":
		fmt.Fprintf(os.Stdout, "handled event type: %s\n", event.Type)
		var stripeType stripe.Product
		err = s.Delete(req.Context(), event.Data.Raw, &stripeType)
		if err != nil {
			klog.Infof("error Delete %T: %v", stripeType, err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unhandled event type: %s\n", event.Type)
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Manager) Serve(addr string) {

	r := mux.NewRouter()
	r.HandleFunc("/stripe", s.handler)

	klog.Fatal(http.ListenAndServe(addr, r))
}
