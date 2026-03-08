package utils

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

func TestGetKnownKindsToWatch(t *testing.T) {
	g := NewWithT(t)
	knownKinds := GetKnownKindsToWatch()
	g.Expect(knownKinds).NotTo(BeEmpty())
	g.Expect(knownKinds).To(HaveKeyWithValue("deployment", ResourceGVK{Group: "apps", Version: "v1"}))
	g.Expect(knownKinds).To(HaveKeyWithValue("secret", ResourceGVK{Group: "", Version: "v1"}))
}

func TestGetKindsToWatchFromConfigMap(t *testing.T) {
	g := NewWithT(t)
	testCases := []struct {
		name     string
		data     map[string]string
		expected []string
	}{
		{"simple case", map[string]string{"kindsToObserve": "Deployment;Secret"}, []string{"Deployment", "Secret"}},
		{"with spaces", map[string]string{"kindsToObserve": " Deployment ; Secret "}, []string{"Deployment", "Secret"}},
		{"empty value", map[string]string{"kindsToObserve": ""}, []string{}},
		{"missing key", map[string]string{}, []string{}},
		{"only semicolons", map[string]string{"kindsToObserve": ";;;"}, []string{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cm := v1.ConfigMap{Data: tc.data}
			result := GetKindsToWatchFromConfigMap(cm)
			if len(tc.expected) == 0 {
				g.Expect(result).To(BeEmpty())
			} else {
				g.Expect(result).To(Equal(tc.expected))
			}
		})
	}
}

func TestGetActionsToWatchFromConfigMap(t *testing.T) {
	g := NewWithT(t)
	testCases := []struct {
		name     string
		data     map[string]string
		expected []string
	}{
		{"simple case", map[string]string{"actionsToObserve": "delete;update"}, []string{"delete", "update"}},
		{"with spaces", map[string]string{"actionsToObserve": " delete ; update "}, []string{"delete", "update"}},
		{"empty value", map[string]string{"actionsToObserve": ""}, []string{}},
		{"missing key", map[string]string{}, []string{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cm := v1.ConfigMap{Data: tc.data}
			result := GetActionsToWatchFromConfigMap(cm)
			if len(tc.expected) == 0 {
				g.Expect(result).To(BeEmpty())
			} else {
				g.Expect(result).To(Equal(tc.expected))
			}
		})
	}
}

func TestGetNamespacesToIgnoreFromConfigMap(t *testing.T) {
	g := NewWithT(t)
	testCases := []struct {
		name     string
		data     map[string]string
		expected []string
	}{
		{"simple case", map[string]string{"namespacesToIgnore": "kube-system;default"}, []string{"kube-system", "default"}},
		{"with spaces", map[string]string{"namespacesToIgnore": " kube-system ; default "}, []string{"kube-system", "default"}},
		{"empty value", map[string]string{"namespacesToIgnore": ""}, []string{}},
		{"missing key", map[string]string{}, []string{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cm := v1.ConfigMap{Data: tc.data}
			result := GetNamespacesToIgnoreFromConfigMap(cm)
			if len(tc.expected) == 0 {
				g.Expect(result).To(BeEmpty())
			} else {
				g.Expect(result).To(Equal(tc.expected))
			}
		})
	}
}

func TestGetTimeConfigFromConfigMap(t *testing.T) {
	g := NewWithT(t)

	cmWithValues := v1.ConfigMap{Data: map[string]string{"minutesToKeep": " 60 ", "hoursToKeep": " 1", "daysToKeep": "7"}}
	g.Expect(GetMinutesToKeepFromConfigMap(cmWithValues)).To(Equal("60"))
	g.Expect(GetHoursToKeepFromConfigMap(cmWithValues)).To(Equal("1"))
	g.Expect(GetDaysToKeepFromConfigMap(cmWithValues)).To(Equal("7"))

	cmWithoutValues := v1.ConfigMap{Data: map[string]string{}}
	g.Expect(GetMinutesToKeepFromConfigMap(cmWithoutValues)).To(BeEmpty())
	g.Expect(GetHoursToKeepFromConfigMap(cmWithoutValues)).To(BeEmpty())
	g.Expect(GetDaysToKeepFromConfigMap(cmWithoutValues)).To(BeEmpty())
}
