package main

import (
        "os"
	"encoding/json"
        "context"
        "time"
	"strconv"
	"crypto/tls"
	"net/http"

        log "github.com/sirupsen/logrus"

        "k8s.io/client-go/kubernetes"
        "k8s.io/client-go/rest"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
        v1 "k8s.io/api/core/v1"

	"github.com/go-co-op/gocron"
	"github.com/gorilla/mux"
	"github.com/eclipse/paho.mqtt.golang"

)


	
type kconfig struct {
	protocol string		`json:"protocol"`
	tag string		`json:"tag"`
	pod string		`json:"pod"`
	event string		`json:"event"`
	url string		`json:"url"`
	httpresponse string	`json:"httpresponse"`
	interval  int		`json:"interval"`
}


type StatusRecorder struct {
	http.ResponseWriter
	Status int
}



var exit = make(chan bool)

var datastore = map[string]string{}




func NewTLSConfig() *tls.Config {
	// Create tls.Config with desired tls properties
	return &tls.Config{
		// RootCAs = certs used to verify server cert.
		//RootCAs: certpool,
		// ClientAuth = whether to request cert from server.
		// Since the server is set up for SSL, this happens
		// anyways.
		//ClientAuth: tls.NoClientCert,
		// ClientCAs = certs used to validate client cert.
		//ClientCAs: nil,
		// InsecureSkipVerify = verify that cert contents
		// match server. IP matches what is in cert etc.
		InsecureSkipVerify: true,
		// Certificates = list of certs client sends to server.
		//Certificates: []tls.Certificate{cert},
	}
}





func e404Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{"message": "not found", "status": "404"}`))
}



func httpHandler(w http.ResponseWriter, r *http.Request) {

	data, err := json.Marshal(datastore)

	if err != nil {
		log.Error(err)
    	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}



var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	log.Info("MQTT: " + string(msg.Topic()) + " - " + string(msg.Payload()))
}


func mq(pod string, url string) {
	tlsconfig := NewTLSConfig()

	opts := mqtt.NewClientOptions()
	opts.AddBroker(url)
	opts.SetClientID("kubehc")
	opts.SetTLSConfig(tlsconfig)
	opts.SetUsername(os.Getenv("MQTT_USER"))
	opts.SetPassword(os.Getenv("MQTT_PASS"))

	opts.SetDefaultPublishHandler(f)

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		log.Error(token.Error())
		log.Info("Check: " + pod + " FAIL")
		datastore[pod] = "fail"
		return
	}


	// Subscribe to a topic
	if token := c.Subscribe("/testtopic/#", 0, nil); token.Wait() && token.Error() != nil {
		log.Error(token.Error())
		log.Info("Check: " + pod + " FAIL")
		datastore[pod] = "fail"
		return
	}
	
	// Publish a message
	token := c.Publish("/testtopic/1", 0, false, pod)
	token.Wait()

	time.Sleep(5 * time.Second)

	// Unscribe
	if token := c.Unsubscribe("/testtopic/#"); token.Wait() && token.Error() != nil {
		log.Error(token.Error())
		log.Info("Check: " + pod + " FAIL")
		datastore[pod] = "fail"
		return
	}
  
	// Disconnect
	c.Disconnect(250)

	time.Sleep(1 * time.Second)
	log.Info("Check: " + pod + " PASS")
	datastore[pod] = "pass"

}



func hc(tag string, url string, pod string, httpresponse string) {
	
	tr := &http.Transport{
        	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
    	}
    
	client := &http.Client{Transport: tr}
    	response, err := client.Get(url)
    	if err != nil {
        	log.Error(err)
		log.Info("Check: " + pod + " FAIL")
		datastore[pod] = "fail"
    	} else {
	    	defer response.Body.Close()
		rc := strconv.Itoa(response.StatusCode)
		log.Info("Check: " + pod + ", " + tag + ", response: " + rc)

		if rc == httpresponse {
			log.Info("Check: " + pod + " PASS")
			datastore[pod] = "pass"
		} else {
			log.Info("Check: " + pod + " FAIL")
			datastore[pod] = "fail"
		}

	    	//content, _ := ioutil.ReadAll(response.Body)
	    	//s := strings.TrimSpace(string(content))
		//log.Info(s)
	}
}




func main() {

        log.Info("Starting Application")
        namespace := os.Getenv("NAMESPACE")

        // creates the in-cluster config
        config, err := rest.InClusterConfig()
        if err != nil {
            panic(err.Error())
        }
        // creates the clientset
        clientset, err := kubernetes.NewForConfig(config)
        if err != nil {
            panic(err.Error())
        }

	kcon := make(chan kconfig)

	s := gocron.NewScheduler(time.UTC)
	s.TagsUnique()


        go func () {
                for {

                        watch, err := clientset.CoreV1().Pods(namespace).Watch(context.TODO(), metav1.ListOptions{})

                        if err != nil {
                                log.Fatal(err.Error())
                        }

                        for event := range watch.ResultChan() {
                                //log.Info("Event Type: " + event.Type)
                                p, ok := event.Object.(*v1.Pod)
                                if !ok {
                                        log.Fatal("Pod: unexpected type")
                                }
                                //log.Info("Pod Name: " + p.Name)
				
				a := kconfig{}
				
				for annotation_name, annotation_value := range p.GetAnnotations(){
					//log.Info(annotation_name)
					//log.Info(annotation_value)
					if annotation_name == "eppo.io/healthcheck-httpresponse" {
						log.Info(p.Name + " - " + annotation_name + ": " + annotation_value)
						a.httpresponse = annotation_value
					} else if annotation_name == "eppo.io/healthcheck-interval" {
						log.Info(p.Name + " - " + annotation_name + ": " + annotation_value)
						intVar, _ := strconv.Atoi(annotation_value)
						a.interval = intVar
					} else if annotation_name == "eppo.io/healthcheck-url" {
						log.Info(p.Name + " - " + annotation_name + ": " + annotation_value)
						a.url = annotation_value
					} else if annotation_name == "eppo.io/healthcheck-tag" {
						log.Info(p.Name + " - " + annotation_name + ": " + annotation_value)
						a.tag = annotation_value
					} else if annotation_name == "eppo.io/healthcheck-protocol" {
						log.Info(p.Name + " - " + annotation_name + ": " + annotation_value)
						a.protocol = annotation_value
					}

				}
				
				// Check if data captured
				if (kconfig{}) != a  {
					log.Info("Annotations Found in " + string(p.Name))
					a.event = string(event.Type)
					a.pod = string(p.Name)
					kcon <- a
				} else {
					// Remove from Scheduler if exists
					//log.Info("Removing " + string(p.Name))
					err = s.RemoveByTag(string(p.Name))
					if err != nil {
				        	log.Error(err)
    					}
				}


                        }
                        log.Info("Secret: Breaking out of loop")
                        time.Sleep(5 * time.Second)
                }
        }()



	go func () {
		//msg := <-kcon
		for msg := range kcon {
			//log.Info("Data Recv")
		
			if msg.event == "ADDED" {
				if msg.protocol == "http" {
					log.Info("Adding HTTP Schedule for " + msg.pod)
					_, err = s.Every(msg.interval).Seconds().Tag(msg.pod).Do(hc, msg.pod, msg.url, msg.pod, msg.httpresponse)
					if err != nil {
        					log.Error(err)
    					}
				} else if msg.protocol == "mqtt" {
					log.Info("Adding MQTT Schedule for " + msg.pod)
					_, err = s.Every(msg.interval).Seconds().Tag(msg.pod).Do(mq, msg.pod, msg.url)
					if err != nil {
        					log.Error(err)
    					}
				}
				datastore[msg.pod] = "pending"
			} else if msg.event == "MODIFIED" {
				log.Info("Updating Schedule for " + msg.pod)
				err = s.RemoveByTag(msg.pod)
				if err != nil {
        				log.Error(err)
    				}
				
				if msg.protocol == "http" {
					log.Info("Adding HTTP Schedule for " + msg.pod)
					_, err = s.Every(msg.interval).Seconds().Tag(msg.pod).Do(hc, msg.pod, msg.url, msg.pod, msg.httpresponse)
					if err != nil {
        					log.Error(err)
    					}
				} else if msg.protocol == "mqtt" {
					log.Info("Adding MQTT Schedule for " + msg.pod)
					_, err = s.Every(msg.interval).Seconds().Tag(msg.pod).Do(mq, msg.pod, msg.url)
					if err != nil {
        					log.Error(err)
    					}
				}
				datastore[msg.pod] = "pending"
			} else if msg.event == "DELETED" {
				log.Info("Removing Schedule for " + msg.pod)
				err = s.RemoveByTag(msg.pod)
				if err != nil {
        				log.Error(err)
    				}
				// Wait 5 seconds to delete
				time.Sleep(5 * time.Second)
				delete(datastore, msg.pod)
			}

		}
	}()



	go func () {
		
		listenAddr := ":8080"
		
		r := mux.NewRouter()

		api := r.PathPrefix("/api").Subrouter()
		api.HandleFunc("/status", httpHandler).Methods(http.MethodGet)

		r.NotFoundHandler = r.NewRoute().HandlerFunc(e404Handler).GetHandler()
		r.Use(loggingMiddleware)

		log.Fatal(http.ListenAndServe(":8080", r))

		server := &http.Server{
			Addr:         listenAddr,
			Handler:      r,
			//ErrorLog:     logger,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  15 * time.Second,
		}
	
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Could not listen on %s: %v\n", listenAddr, err)
		}

	}()

	

	s.StartAsync()

        <-exit

}




func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &StatusRecorder{
			ResponseWriter: w,
			Status:         200,
		}
		next.ServeHTTP(recorder, r)

		// Do stuff here
		log.Info(r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent(), recorder.Status)
		// Call the next handler, which can be another middleware in the chain, or the final handler.
		//next.ServeHTTP(w, r)
	})
}


func (r *StatusRecorder) WriteHeader(status int) {
	r.Status = status
	r.ResponseWriter.WriteHeader(status)
}


