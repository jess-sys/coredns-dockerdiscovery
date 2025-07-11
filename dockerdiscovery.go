package dockerdiscovery

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/coredns/coredns/plugin"
	dockerapi "github.com/fsouza/go-dockerclient"
)

// DockerDiscovery is a plugin that conforms to the coredns plugin interface
type DockerDiscovery struct {
	Next           plugin.Handler
	dockerEndpoint string
	dockerClient   *dockerapi.Client
	mutex          sync.RWMutex
	serviceInfo    ServiceInfoMap
	ttl            uint32
}

// NewDockerDiscovery constructs a new DockerDiscovery object
func NewDockerDiscovery(dockerEndpoint string) *DockerDiscovery {
	return &DockerDiscovery{
		dockerEndpoint: dockerEndpoint,
		serviceInfo:    make(ServiceInfoMap),
		ttl:            3600,
	}
}

// Name implements plugin.Handler
func (dd *DockerDiscovery) Name() string {
	return "docker"
}

func (dd *DockerDiscovery) start() error {
	log.Println("[swarmdiscovery] start")
	events := make(chan *dockerapi.APIEvents)

	if err := dd.dockerClient.AddEventListener(events); err != nil {
		return err
	}

	services, err := dd.dockerClient.ListServices(dockerapi.ListServicesOptions{})
	if err != nil {
		log.Printf("[swarmdiscovery] Error listing services: %s", err)
	} else {
		log.Printf("[swarmdiscovery] Found %d services", len(services))
		for _, service := range services {
			// log.Printf("[docker] Service %s %+v\n\n", service.Spec.Name, service.Spec.Labels)
			hostnames := dd.GetHostnamesFromLabels(&service)
			for _, hostname := range hostnames {
				log.Printf("[swarmdiscovery] Registering service %s to host: %s\n", service.Spec.Name, hostname)
				if err := dd.updateServiceInfo(&service); err != nil {
					log.Printf("[swarmdiscovery] Error adding CNAME record for service %s: %s\n", service.ID[:12], err)
				}
			}
		}
	}

	for msg := range events {
		go func(msg *dockerapi.APIEvents) {
			event := fmt.Sprintf("%s:%s", msg.Type, msg.Action)
			fmt.Printf("[special] new API event of type %s: %s, full dump: %+v\n", msg.Type, event, msg)
			switch event {
			case "service:create":
				tasks, err := dd.dockerClient.ListTasks(dockerapi.ListTasksOptions{
					Filters: map[string][]string{
						"node": {msg.Actor.ID},
					},
				})
				if err != nil {

				} else {
					log.Printf("\n\nNew task spawned for node %s:\n %+v\n\n", msg.Actor.Attributes["name"], tasks)
				}
			case "service:remove":
			}
		}(msg)
	}

	return errors.New("docker event loop closed")
}
