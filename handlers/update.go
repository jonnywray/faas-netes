package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/openfaas/faas/gateway/requests"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"strconv"
)

// MakeUpdateHandler update specified function
func MakeUpdateHandler(functionNamespace string, clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		body, _ := ioutil.ReadAll(r.Body)

		request := requests.CreateFunctionRequest{}
		err := json.Unmarshal(body, &request)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		getOpts := metav1.GetOptions{}

		deployment, findDeployErr := clientset.ExtensionsV1beta1().Deployments(functionNamespace).Get(request.Service, getOpts)
		if findDeployErr != nil {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(findDeployErr.Error()))
			return
		}

		if len(deployment.Spec.Template.Spec.Containers) > 0 {
			deployment.Spec.Template.Spec.Containers[0].Image = request.Image
			deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = v1.PullAlways

			deployment.Spec.Template.Spec.Containers[0].Env = buildEnvVars(&request)

			deployment.Spec.Template.Spec.NodeSelector = createSelector(request.Constraints)

			labels := map[string]string{
				"faas_function": request.Service,
				"uid":           fmt.Sprintf("%d", time.Now().Nanosecond()),
			}

			if request.Labels != nil {
				for k, v := range *request.Labels {
					if k == "com.openfaas.scale.min" {
						minReplicas, err := strconv.Atoi(v)
						if err != nil && minReplicas > 0 {
							deployment.Spec.Replicas = int32p(int32(minReplicas))
						}
					}
					labels[k] = v
				}
			}

			deployment.Spec.Template.Labels = labels
			deployment.Spec.Template.ObjectMeta.Labels = labels

			resources, resourceErr := createResources(request)
			if resourceErr != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(resourceErr.Error()))
				return
			}

			deployment.Spec.Template.Spec.Containers[0].Resources = *resources
		}

		if _, updateErr := clientset.ExtensionsV1beta1().Deployments(functionNamespace).Update(deployment); updateErr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(updateErr.Error()))
		}
	}
}
