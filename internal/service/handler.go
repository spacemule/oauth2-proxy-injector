// package service

// import (
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"net/http"

// 	admissionv1 "k8s.io/api/admission/v1"
// 	corev1 "k8s.io/api/core/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/runtime"
// 	"k8s.io/apimachinery/pkg/runtime/serializer"
// 	"k8s.io/klog/v2"
// )

// var (
// 	runtimeScheme = runtime.NewScheme()
// 	codecs        = serializer.NewCodecFactory(runtimeScheme)
// 	deserializer  = codecs.UniversalDeserializer()
// )

// func init() {
// 	_ = corev1.AddToScheme(runtimeScheme)
// 	_ = admissionv1.AddToScheme(runtimeScheme)
// }

// // Handler handles admission requests for Service resources
// type Handler struct {
// 	mutator Mutator
// }

// // NewHandler creates a new admission Handler for Services
// func NewHandler(mutator Mutator) *Handler {
// 	return &Handler{
// 		mutator: mutator,
// 	}
// }

// // HandleAdmission processes a Service admission request
// //
// // TODO: Implement this function (similar to pod admission handler)
// // 1. Read request body
// // 2. Deserialize AdmissionReview
// // 3. Validate it's a Service resource
// // 4. Deserialize Service from AdmissionRequest.Object
// // 5. Call mutator.Mutate(ctx, service)
// // 6. If patches returned, create JSONPatch response
// // 7. If error, create failed AdmissionResponse with error message
// // 8. Write AdmissionReview response
// //
// // Note: This is very similar to internal/admission/handler.go
// // Consider extracting common logic into a shared package if duplication is significant
// func (h *Handler) HandleAdmission(w http.ResponseWriter, r *http.Request) {
// 	panic("TODO: implement")
// }

// // createAdmissionResponse creates an AdmissionResponse for a successful mutation
// //
// // TODO: Implement this function
// // 1. If patches is empty, return Allowed=true with no patch
// // 2. If patches exist:
// //    a. Marshal patches to JSON
// //    b. Set PatchType to JSONPatch
// //    c. Set Patch to marshaled bytes
// // 3. Return AdmissionResponse
// func createAdmissionResponse(uid string, patches []byte) *admissionv1.AdmissionResponse {
// 	panic("TODO: implement")
// }

// // createErrorResponse creates an AdmissionResponse for a failed mutation
// //
// // TODO: Implement this function
// // 1. Return AdmissionResponse with:
// //    - UID set
// //    - Allowed = false
// //    - Result with Status=Failure and Message=error
// func createErrorResponse(uid string, err error) *admissionv1.AdmissionResponse {
// 	panic("TODO: implement")
// }

// // writeResponse writes an AdmissionReview response
// //
// // TODO: Implement this function
// // 1. Create AdmissionReview with response
// // 2. Set Content-Type header
// // 3. Marshal and write response
// func writeResponse(w http.ResponseWriter, response *admissionv1.AdmissionResponse) {
// 	panic("TODO: implement")
// }

// // Suppress unused import errors during scaffolding
// var (
// 	_ = json.Marshal
// 	_ = fmt.Sprintf
// 	_ = io.ReadAll
// 	_ = http.MethodPost
// 	_ = admissionv1.SchemeGroupVersion
// 	_ = metav1.Now
// 	_ = klog.Info
// )
