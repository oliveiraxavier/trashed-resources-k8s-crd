package utils

import (
	"strings"

	v1 "k8s.io/api/core/v1"
)

// ResourceGVK define a estrutura para mapear o Group e Version de um Kind
type ResourceGVK struct {
	Group   string
	Version string
}

var knownGVKs = map[string]ResourceGVK{
	"deployment":  {Group: "apps", Version: "v1"},
	"secret":      {Group: "", Version: "v1"},
	"configmap":   {Group: "", Version: "v1"},
	"statefulset": {Group: "apps", Version: "v1"},
	"daemonset":   {Group: "apps", Version: "v1"},
	"ingress":     {Group: "networking.k8s.io", Version: "v1"},
	"cronjob":     {Group: "batch", Version: "v1"},
	"job":         {Group: "batch", Version: "v1"},
	"service":     {Group: "", Version: "v1"},
}

func GetKnownKindsToWatch() map[string]ResourceGVK {
	return knownGVKs
}

func GetKindsToWatchFromConfigMap(configMapData v1.ConfigMap) []string {
	rawKinds := strings.Split(configMapData.Data["kindsTobserve"], ";")

	return strings.Fields(strings.Join(rawKinds, " "))
}

func GetActionsToWatchFromConfigMap(configMapData v1.ConfigMap) []string {
	rawActions := strings.Split(configMapData.Data["actionsToObserve"], ";")

	return strings.Fields(strings.Join(rawActions, " "))
}

func GetNamespacesToIgnoreFromConfigMap(configMapData v1.ConfigMap) []string {
	rawNamespaces := strings.Split(configMapData.Data["namespacesToIgnore"], ";")

	return strings.Fields(strings.Join(rawNamespaces, " "))
}

func GetMinutesToKeepFromConfigMap(configMapData v1.ConfigMap) string {
	return strings.Join(strings.Fields(configMapData.Data["minutesToKeep"]), " ")
}

func GetHoursToKeepFromConfigMap(configMapData v1.ConfigMap) string {
	return strings.Join(strings.Fields(configMapData.Data["hoursToKeep"]), " ")
}

func GetDaysToKeepFromConfigMap(configMapData v1.ConfigMap) string {
	return strings.Join(strings.Fields(configMapData.Data["daysToKeep"]), " ")
}
