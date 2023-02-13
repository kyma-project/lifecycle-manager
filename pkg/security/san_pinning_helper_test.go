package security_test

import (
	"io"
	"net/http"
	"strings"

	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/kyma-project/lifecycle-manager/pkg/security"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createKyma(kymaName string, annotations map[string]string) *v1beta1.Kyma {
	return &v1beta1.Kyma{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1beta1.GroupVersion.String(),
			Kind:       string(v1beta1.KymaKind),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        kymaName,
			Namespace:   v1.NamespaceDefault,
			Annotations: annotations,
		},
		Spec: v1beta1.KymaSpec{
			Modules: []v1beta1.Module{},
			Channel: v1beta1.DefaultChannel,
		},
	}
}

func createRequest(kymaName string, header map[string][]string) *http.Request {
	return &http.Request{
		Body: io.NopCloser(
			strings.NewReader("{\n    \"owner\": {\n        " +
				"\"Namespace\": \"default\",\n        " +
				"\"Name\": \"" + kymaName + "\"\n    },\n    " +
				"\"watched\": {\n        " +
				"\"Namespace\": \"default\",\n        " +
				"\"Name\": \"" + kymaName + "\"\n    },\n    " +
				"\"watchedGvk\": {\n        " +
				"\"group\": \"testGroup\",\n        " +
				"\"version\": \"testResourceVersion\",\n        " +
				"\"kind\": \"testResourceKind\"\n    }\n}"),
		),
		Header: header,
	}
}

func createWatcherCR(kymaName string) *types.WatchEvent {
	return &types.WatchEvent{
		Owner: client.ObjectKey{
			Namespace: "default",
			Name:      kymaName,
		},
		Watched: client.ObjectKey{
			Namespace: "default",
			Name:      kymaName,
		},
		WatchedGvk: metav1.GroupVersionKind{
			Group:   "testGroup",
			Version: "testResourceVersion",
			Kind:    "testResourceKind",
		},
	}
}

var (
	annotationsWithCorrectDomain = map[string]string{"skr-domain": "example.domain.com"} //nolint:gochecknoglobals
	annotationsWithWrongDomain   = map[string]string{"skr-domain": "wrong.domain.com"}   //nolint:gochecknoglobals
	emptyAnnotations             = map[string]string{}                                   //nolint:gochecknoglobals

	headerWithSufficientCertificate = map[string][]string{ //nolint:gochecknoglobals
		security.XFCCHeader: {
			"Hash=d54ce461112371914142ca640f0ff7edb5b47778e1e97185336b79f4590e2ce4;Cert=\"" +
				"-----BEGIN%20CERTIFICATE-----%0AMIIDRzCCAi%2BgAwIBAgIBATANBgkqhkiG9w0BAQs" +
				"FADA0MRUwEwYDVQQKDAxleGFt%0AcGxlIEluYy4xGzAZBgNVBAMMEmV4YW1wbGUuZG9tYWluL" +
				"mNvbTAeFw0yMjEyMDgx%0AMDI5NDBaFw0yMzEyMDgxMDI5NDBaMEYxCzAJBgNVBAYTAlhYMQ0" +
				"wCwYDVQQHDARD%0AaXR5MQswCQYDVQQKDAJCVTEbMBkGA1UEAwwSZXhhbXBsZS5kb21haW4uY" +
				"29tMIIB%0AIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAxZFDk5hQyKOhrUKeuapTWpN" +
				"w%0Ax%2F4V54%2Fovd8tMwZ60xMBoOgH3w5l32%2F%2BuaYioKAAS%2F1FrJDhE6KO6UnW8sO" +
				"h%2BeAf%0AoCNgyn7R0H%2BU9eIsWTpN8czvF6Nh1%2FV0Hj6HKKo995RNQqwB6wVo%2Bzqdp" +
				"fyOXoeC%0A6HSvWNvyecq28OkAWyVxhFGzlp8foPQkbrNjSMHUQyFuM7esJI3EJ8fS2yKHgAo" +
				"D%0AyH%2FV3AgSBNEPlUi9TDg0Aumz2uSG9bOi%2Fz%2B2ytOrblR5MdlAKsZv3AzeaChJRo3" +
				"%2B%0AnPrfrCsdykbM%2B87VGhCi4Aj7wg4SiXGywp9uV6XVINZ%2FeuD86Pqqo526kYVCgwI" +
				"D%0AAQABo1IwUDBOBgNVHREERzBFghJleGFtcGxlLmRvbWFpbi5jb22CGWNsaWVudC5l%0AeG" +
				"FtcGxlLmRvbWFpbi5jb22CFCouZXhhbXBsZS5kb21haW4uY29tMA0GCSqGSIb3%0ADQEBCwUA" +
				"A4IBAQBkdmaZUg6UGB5mICKTF65bUM0Wje4dpgUshrXBDGoq0%2FR2WHil%0AyG24GViLbYhw" +
				"dHoM%2BiAqNdgjdsxr6LM38Z0MBVOni7QmMQZh2ojocRuvY9k7Kf6O%0A%2BcRv5QCxu9xEYD" +
				"cPDsxWf1873gUcxOQhxsgNXmG4D6WKFTSmscfOWn%2FISAY5lsN4%0ApLsOFr%2BLIm6os07i" +
				"IbF4n3urLc9UWfliNkE1rOl60BjrPdGmdscjw6EW63ixzVpq%0A57yMAk2l1vIUSlCSQqwCXh" +
				"TlWAd8DkgNZDM9Zcr%2BvK0C7uh6qUc%2FrHh2%2BVg6wTQb%0AvqVmzWOq64A3Jp10o7zzR3" +
				"%2BNJSUVl07v52f8%0A-----END%20CERTIFICATE-----%0A\";Subject=\"CN=example." +
				"domain.com,O=BU,L=City,C=XX\";URI=;DNS=example.domain.com;DNS=client.exam" +
				"ple.domain.com;DNS=*.example.domain.com",
		},
	}
	headerWithMalformedCertificate = map[string][]string{ //nolint:gochecknoglobals
		security.XFCCHeader: {
			"Hash=d54ce461112371914142ca640f0ff7edb5b47778e1e97185336b79f4590e2ce4;Cert=\"" +
				"-----BEGIN%20CERTIFICATE-----%0WRONGRzCCAi%2BgAwIBAgIBATANBgkqhkiG9w0BAQs" +
				"FADA0MRUwEwYDVQQKDAxleGFt%0AcGxlIEluYy4xGzAZBgNVBAMMEmV4YW1wbGUuZG9tYWluL" +
				"mNvbTAeFw0yMjEyMDgx%0AMDI5NDBaFw0yMzEyMDgxMDI5NDBaMEYxCzAJBgNVBAYTAlhYMQ0" +
				"wCwYDVQQHDARD%0AaXR5MQswCQYDVQQKDAJCVTEbMBkGA1UEAwwSZXhhbXBsZS5kb21haW4uY" +
				"29tMIIB%0AIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAxZFDk5hQyKOhrUKeuapTWpN" +
				"w%0Ax%2F4V54%2Fovd8tMwZ60xjabnaH3w5l32%2F%2BuaYioKAAS%2F1FrJDhE6KO6UnW8sO" +
				"h%2BeAf%0AoCNgyn7R0H%2BU9eIsWTpN8czvF6Nh1%2FV0Hj6HKKo995RNQqwB6wVo%2Bzqdp" +
				"fyOXoeC%0A6HSvWNvyecq28OkAWyVxhFGzlp8foPQkbrNjSMHUQyFuM7esJI3EJ8fS2yKHgAo" +
				"D%0AyH%2FV3AgSBNEPlUi9TDg0Aumz2uSG9bOi%2Fz%2B2ytOrblR5MdlAKsZv3AzeaChJRo3" +
				"%2B%0AnPrfrCsdykbM%2B87VGhCi4Aj7wg4SiXGywp9uV6XVINZ%2FeuD86Pqqo526kYVCgwI" +
				"D%0AAQABo1IwUDBOBgNVHREERzBFghJleGFtcGxlLmRvbWFpbi5jb22CGWNsaWVudC5l%0AeG" +
				"FtcGxlLmRvbWFpbi5jb22CFCouZXhhbXBsZS5kb21haW4uY29tMA0GCSqGSIb3%0ADQEBCwUA" +
				"A4IBAQBkdmaZUg6UGB5mICKTF65bUM0Wje4dpgUshrXBDGoq0%2FR2WHil%0AyG24GViLbYhw" +
				"dHoM%2BiAqNdgjdsxr6LM38Z0MBVOni7QmMQZh2ojocRuvY9k7Kf6O%0A%2BcRv5QCxu9xEYD" +
				"cPDsxWf1873gUcxOQhxsgNXmG4D6WKFTSmscfOWn%2FISAY5lsN4%0ApLsOFr%2BLIm6os07i" +
				"IbF4n3urLc9UWfliNkE1rOl60BjrPdGmdscjw6EW63ixzVpq%0A57yMAk2l1vIUSlCSQqwCXh" +
				"TlWAd8DkgNZDM9Zcr%2BvK0C7uh6qUc%2FrHh2%2BVg6wTQb%0AvqVmzWOq64A3Jp10o7zzR3" +
				"%2BNJSUVl07v52f8%0A-----END%20CERTIFICATE-----%0A\";Subject=\"CN=example." +
				"domain.com,O=BU,L=City,C=XX\";URI=;DNS=example.domain.com;DNS=client.exam" +
				"ple.domain.com;DNS=*.example.domain.com",
		},
	}
)
