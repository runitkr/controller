package main

import (
	"context"
	"math/rand"

	"github.com/gofiber/fiber/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
		runnerId := randStringRunes(10)

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
						RuntimeClassName: ptr.To("gvisor"),
						Containers: []corev1.Container{
							{
								Name:  "codeserver",
								Image: "ghcr.io/coder/code-server:latest",
								Args: []string{
									"--auth=none",
									"--disable-telemetry",
								},
								Resources: corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:              resource.MustParse("100m"),
										corev1.ResourceMemory:           resource.MustParse("100Mi"),
										corev1.ResourceEphemeralStorage: resource.MustParse("10Gi"),
									},
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
			},
			Spec: netv1.IngressSpec{
				IngressClassName: ptr.To("nginx"),
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

		_, err = ingressCtx.Create(context.TODO(), &ingress, metav1.CreateOptions{})
		if err != nil {
			panic(err.Error())
		}

		_, err = serviceCtx.Create(context.TODO(), &service, metav1.CreateOptions{})
		if err != nil {
			panic(err.Error())
		}

		_, err = deploymentCtx.Create(context.TODO(), &deployment, metav1.CreateOptions{})
		if err != nil {
			panic(err.Error())
		}

		return c.JSON(fiber.Map{
			"success":  true,
			"runnerId": runnerId,
		})
	})

	app.Listen(":8080")
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
