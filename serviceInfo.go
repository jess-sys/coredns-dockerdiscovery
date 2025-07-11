package dockerdiscovery

import (
	"fmt"
	"github.com/docker/docker/api/types/swarm"
	"log"
	"strings"
)

type ServiceInfo struct {
	service   *swarm.Service
	hostnames []string
	worker    string
}

type ServiceInfoMap map[string]*ServiceInfo

func (dd *DockerDiscovery) GetHostnamesFromLabels(service *swarm.Service) []string {
	hostnames := []string{}
	for labelKey, labelValue := range service.Spec.Labels {
		if strings.HasPrefix(labelKey, "coredns.hostname.") {
			hostnames = append(hostnames, labelValue)
		}
	}
	return hostnames
}

func (dd *DockerDiscovery) GetWorkerFromLabels(service *swarm.Service) string {
	for labelKey, labelValue := range service.Spec.Labels {
		if labelKey == "coredns.worker" {
			return labelValue
		}
	}
	return ""
}

func (dd *DockerDiscovery) serviceInfoByHostname(requestName string) (*ServiceInfo, error) {
	dd.mutex.RLock()
	defer dd.mutex.RUnlock()

	for _, serviceInfo := range dd.serviceInfo {
		for _, d := range serviceInfo.hostnames {
			if fmt.Sprintf("%s.", d) == requestName { // qualified domain name must be specified with a trailing dot
				return serviceInfo, nil
			}
		}
	}

	return nil, nil
}

func (dd *DockerDiscovery) updateServiceInfo(service *swarm.Service) error {
	dd.mutex.Lock()
	defer dd.mutex.Unlock()

	_, isExist := dd.serviceInfo[service.ID]
	if isExist { // remove previous resolved container info
		delete(dd.serviceInfo, service.ID)
	}

	worker := dd.GetWorkerFromLabels(service)
	hostnames := dd.GetHostnamesFromLabels(service)
	if len(hostnames) > 0 {
		dd.serviceInfo[service.ID] = &ServiceInfo{
			service:   service,
			hostnames: hostnames,
			worker:    worker,
		}

		if !isExist {
			log.Printf("[swarmdiscovery] Add service entry %s (%s)", service.Spec.Name, service.ID[:12])
		}
	} else if isExist {
		log.Printf("[swarmdiscovery] Remove service entry %s (%s)", service.Spec.Name, service.ID[:12])
	}
	return nil
}

func (dd *DockerDiscovery) removeServiceInfo(serviceID string) error {
	dd.mutex.Lock()
	defer dd.mutex.Unlock()

	serviceInfo, ok := dd.serviceInfo[serviceID]
	if !ok {
		log.Printf("[swarmdiscovery] No entry associated with the container %s", serviceID[:12])
		return nil
	}
	log.Printf("[swarmdiscovery] Deleting service entry %s (%s)", serviceInfo.service.Spec.Name, serviceInfo.service.ID[:12])
	delete(dd.serviceInfo, serviceID)

	return nil
}
