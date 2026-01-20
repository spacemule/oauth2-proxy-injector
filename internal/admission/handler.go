package admission

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

	"github.com/spacemule/oauth2-proxy-injector/internal/mutation"
)

// ContentTypeJSON is the expected Content-Type for admission reviews
const ContentTypeJSON = "application/json"

// Handler handles Kubernetes admission webhook requests
type Handler struct {
	mutator mutation.Mutator
}

// NewHandler creates a new admission Handler
func NewHandler(mutator mutation.Mutator) *Handler {
	return &Handler{
		mutator: mutator,
	}
}

// HandleAdmission is the HTTP handler for /mutate endpoint
// This is the main entry point for admission webhook requests
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
	pod := &corev1.Pod{}

	if request.UID == "" {
		return denied("", "UID not set")
	}

	if request.Kind.Group != "" || request.Kind.Version != "v1" || request.Kind.Kind != "Pod" {
		return allowed(string(request.UID))
	}

	if err := json.Unmarshal(request.Object.Raw, pod); err != nil {
		return denied(string(request.UID), fmt.Sprintf("failed to unmarshal pod: %v", err))
	}

	if request.Operation != admissionv1.Create {
		return allowed(string(request.UID))
	}

	klog.InfoS("processing admission request",
		"pod", pod.Name,
		"namespace", request.Namespace,
		"operation", request.Operation,
	)

	patches, err := h.mutator.Mutate(ctx, pod)
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
