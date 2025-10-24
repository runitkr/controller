package main

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
)

func main() {
	app := fiber.New()

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	namespace := "runners"

	deploymentCtx := clientset.AppsV1().Deployments(namespace)
	serviceCtx := clientset.CoreV1().Services(namespace)
	ingressCtx := clientset.NetworkingV1().Ingresses(namespace)

	app.Static("/", "./public")
	app.Post("/runners", func(c *fiber.Ctx) error {
		runnerId := uuid.New().String()

		deployment := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      runnerId,
				Namespace: namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To[int32](1),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "runner",
						"id":  runnerId,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "runner",
							"id":  runnerId,
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "codeserver",
								Image: "ghcr.io/coder/code-server:latest",
								Args: []string{
									"--auth=none",
									"--disable-telemetry",
								},
							},
						},
					},
				},
			},
		}

		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      runnerId,
				Namespace: namespace,
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Port:       8080,
						TargetPort: intstr.FromInt(8080),
						Protocol:   corev1.ProtocolTCP,
						Name:       "http",
					},
				},
				Selector: map[string]string{
					"app": "runner",
					"id":  runnerId,
				},
			},
		}

		hostname := runnerId + ".run.it.kr"
		ingress := netv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      runnerId,
				Namespace: namespace,
				Annotations: map[string]string{
					"cert-manager.io/cluster-issuer": "letsencrypt",
				},
			},
			Spec: netv1.IngressSpec{
				IngressClassName: ptr.To("nginx"),
				TLS: []netv1.IngressTLS{
					{
						Hosts: []string{
							hostname,
						},
					},
				},
				Rules: []netv1.IngressRule{
					{
						Host: hostname,
						IngressRuleValue: netv1.IngressRuleValue{
							HTTP: &netv1.HTTPIngressRuleValue{
								Paths: []netv1.HTTPIngressPath{
									{
										Path:     "/",
										PathType: ptr.To(netv1.PathType("Prefix")),
										Backend: netv1.IngressBackend{
											Service: &netv1.IngressServiceBackend{
												Name: runnerId,
												Port: netv1.ServiceBackendPort{
													Number: 8080,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		deploymentCtx.Create(context.TODO(), &deployment, metav1.CreateOptions{})
		serviceCtx.Create(context.TODO(), &service, metav1.CreateOptions{})
		ingressCtx.Create(context.TODO(), &ingress, metav1.CreateOptions{})

		return c.JSON(fiber.Map{
			"success":  true,
			"runnerId": runnerId,
		})
	})

	app.Listen(":8080")
}
