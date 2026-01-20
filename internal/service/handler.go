package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// Handler handles admission requests for Service resources
type Handler struct {
	mutator Mutator
}

// NewHandler creates a new admission Handler for Services
func NewHandler(mutator Mutator) *Handler {
	return &Handler{
		mutator: mutator,
	}
}

// HandleAdmission processes a Service admission request
func (h *Handler) HandleAdmission(w http.ResponseWriter, r *http.Request) {
	var review admissionv1.AdmissionReview

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Unsupported Media Type", http.StatusUnsupportedMediaType)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1*1024*1024))
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	err = json.Unmarshal(body, &review)
	if err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}

	if review.Request == nil {
		review.Response = denied("", "missing request in AdmissionReview")
		writeAdmissionReview(w, &review)
		return
	}

	resp := h.handleAdmissionRequest(r.Context(), review.Request)
	review.Response = resp
	writeAdmissionReview(w, &review)
}

// handleAdmissionRequest processes a single admission request
func (h *Handler) handleAdmissionRequest(ctx context.Context, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	svc := &corev1.Service{}

	if request.UID == "" {
		return denied("", "UID not set")
	}

	if request.Kind.Group != "" || request.Kind.Version != "v1" || request.Kind.Kind != "Service" {
		return allowed(string(request.UID))
	}

	if err := json.Unmarshal(request.Object.Raw, svc); err != nil {
		return denied(string(request.UID), fmt.Sprintf("failed to unmarshal service: %v", err))
	}

	if request.Operation != admissionv1.Create {
		return allowed(string(request.UID))
	}

	klog.InfoS("processing admission request",
		"service", svc.Name,
		"namespace", request.Namespace,
		"operation", request.Operation,
	)

	patches, err := h.mutator.Mutate(ctx, svc)
	if err != nil {
		return denied(string(request.UID), err.Error())
	}
	if len(patches) == 0 {
		return allowed(string(request.UID))
	}

	jsonPatches, err := json.Marshal(patches)
	if err != nil {
		return denied(string(request.UID), err.Error())
	}

	return patchResponse(string(request.UID), jsonPatches)

}

// allowed returns an AdmissionResponse allowing the request
func allowed(uid string) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		UID:     types.UID(uid),
		Allowed: true,
	}
}

// denied returns an AdmissionResponse denying the request
func denied(uid string, message string) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		UID:     types.UID(uid),
		Allowed: false,
		Result: &metav1.Status{
			Message: message,
		},
	}
}

// patchResponse returns an AdmissionResponse with a JSON patch
// Takes already-marshaled patch bytes
func patchResponse(uid string, patch []byte) *admissionv1.AdmissionResponse {
	pt := admissionv1.PatchTypeJSONPatch
	return &admissionv1.AdmissionResponse{
		UID:       types.UID(uid),
		Allowed:   true,
		PatchType: &pt,
		Patch:     patch,
	}
}

// writeAdmissionReview writes an AdmissionReview response
// Note: Must write the full AdmissionReview, not just AdmissionResponse
func writeAdmissionReview(w http.ResponseWriter, review *admissionv1.AdmissionReview) {
	body, err := json.Marshal(review)
	if err != nil {
		http.Error(w, "could not marshal review", 500)
		return
	}
	w.Header().Add("content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}
