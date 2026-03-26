package mutation

import (
	corev1 "k8s.io/api/core/v1"
)

// CSI volume mount path for secret provider class files
const SecretProviderMountPath = "/etc/oauth2-proxy/conf.d"

// Volume name for the CSI secrets volume
const SecretProviderVolumeName = "oauth2-proxy-secrets"

// SecretFileKeys are the keys that use --*-file flags instead of passing
// the value directly. These contain sensitive data that should not appear
// in process arguments.
//
// File name -> oauth2-proxy flag
var SecretFileKeys = map[string]string{
	"client-secret": "--client-secret-file",
	"cookie-secret": "--cookie-secret-file",
}

// BuildCSIVolume creates the CSI volume definition for the secret provider class
func BuildCSIVolume(secretProviderClassName string) corev1.Volume {
	readOnly := true
	return corev1.Volume {
		Name: SecretProviderVolumeName,
		VolumeSource: corev1.VolumeSource{
			CSI: &corev1.CSIVolumeSource{
				Driver: "secrets-store.csi.k8s.io",
				ReadOnly: &readOnly,
				VolumeAttributes: map[string]string{
					"secretProviderClass": secretProviderClassName,
				},
			},
		},
	}
}

// BuildCSIVolumeMount creates the volume mount for the oauth2-proxy container
func BuildCSIVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name: SecretProviderVolumeName,
		MountPath: SecretProviderMountPath,
		ReadOnly: true,
	}
}

// GetFileOverridePath returns the file path for a given config key
// within the secret provider mount.
func GetFileOverridePath(key string) string {
	return SecretProviderMountPath + "/" + key
}

// IsSecretKey returns true if the key is a sensitive value that should
// use --*-file flags instead of direct value passing.
func IsSecretKey(key string) bool {
	_, ok := SecretFileKeys[key]
	return ok
}
