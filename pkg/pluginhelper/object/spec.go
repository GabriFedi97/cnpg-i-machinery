/*
Copyright The CloudNativePG Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package object

import (
	"errors"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

const (
	pluginVolumeName = "plugins"
	pluginMountPath  = "/plugins"

	postgresContainerName = "postgres"
)

// ErrNoPostgresContainerFound is raised when there's no PostgreSQL container
// in the passed instance Pod.
var ErrNoPostgresContainerFound = errors.New("no postgres container into instance Pod")

// InjectPluginVolume injects the plugin volume into a CNPG Pod.
func InjectPluginVolume(pod *corev1.Pod) {
	InjectPluginVolumeSpec(&pod.Spec)
}

// InjectPluginVolumeSpec injects the plugin volume into a CNPG Pod spec.
func InjectPluginVolumeSpec(spec *corev1.PodSpec) {
	foundPluginVolume := false
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == pluginVolumeName {
			foundPluginVolume = true
		}
	}

	if foundPluginVolume {
		return
	}

	spec.Volumes = append(spec.Volumes, corev1.Volume{
		Name: pluginVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	for i := range spec.Containers {
		if spec.Containers[i].Name == postgresContainerName {
			spec.Containers[i].VolumeMounts = append(
				spec.Containers[i].VolumeMounts,
				corev1.VolumeMount{
					Name:      pluginVolumeName,
					MountPath: pluginMountPath,
				},
			)
		}
	}
}

// InjectPluginSidecar refer to InjectPluginSidecarSpec.
func InjectPluginSidecar(pod *corev1.Pod, sidecar *corev1.Container, injectPostgresVolumeMounts bool) error {
	return InjectPluginSidecarSpec(&pod.Spec, sidecar, injectPostgresVolumeMounts)
}

// InjectPluginSidecarInitContainer refer to InjectPluginSidecarInitContainerSpec.
func InjectPluginSidecarInitContainer(pod *corev1.Pod,
	sidecar *corev1.Container,
	injectPostgresVolumeMounts bool,
) error {
	return InjectPluginInitContainerSidecarSpec(&pod.Spec, sidecar, injectPostgresVolumeMounts)
}

// InjectPluginSidecarSpec injects a plugin sidecar into a CNPG Pod spec.
//
// If the "injectPostgresVolumeMount" flag is true, this will append all the volume
// mounts that are used in the instance manager Pod to the passed sidecar
// container, granting it superuser access to the PostgreSQL instance.
//
// Besides the value of "injectPostgresVolumeMount", the plugin volume
// will always be injected in the PostgreSQL container.
func InjectPluginSidecarSpec(spec *corev1.PodSpec, sidecar *corev1.Container, injectPostgresVolumeMounts bool) error {
	if sidecar == nil || spec == nil {
		return nil
	}

	return injectPluginSpec(spec, sidecar, &spec.Containers, injectPostgresVolumeMounts)
}

// InjectPluginInitContainerSidecarSpec injects a plugin sidecar into a CNPG Pod spec as
// an InitContainer. This requires the SidecarContainers feature gate to be enabled, which is
// the default for kubernetes versions >= 1.29.
//
// If the "injectPostgresVolumeMount" flag is true, this will append all the volume
// mounts that are used in the instance manager Pod to the passed sidecar
// container, granting it superuser access to the PostgreSQL instance.
//
// Besides the value of "injectPostgresVolumeMount", the plugin volume
// will always be injected in the PostgreSQL container.
func InjectPluginInitContainerSidecarSpec(
	spec *corev1.PodSpec, sidecar *corev1.Container, injectPostgresVolumeMounts bool,
) error {
	if sidecar == nil || spec == nil {
		return nil
	}
	if spec.InitContainers == nil {
		spec.InitContainers = make([]corev1.Container, 0)
	}
	sidecar.RestartPolicy = ptr.To(corev1.ContainerRestartPolicyAlways)

	return injectPluginSpec(spec, sidecar, &spec.InitContainers, injectPostgresVolumeMounts)
}

func injectPluginSpec(spec *corev1.PodSpec,
	sidecar *corev1.Container,
	targetContainerList *[]corev1.Container,
	injectPostgresVolumeMounts bool,
) error {
	if spec == nil || sidecar == nil {
		return nil
	}
	sidecar = sidecar.DeepCopy()
	InjectPluginVolumeSpec(spec)

	var volumeMounts []corev1.VolumeMount
	postgresContainerFound := false
	for i := range spec.Containers {
		if spec.Containers[i].Name == postgresContainerName {
			volumeMounts = spec.Containers[i].VolumeMounts
			postgresContainerFound = true
			break
		}
	}
	containers := *targetContainerList
	sidecarContainerFound := slices.ContainsFunc(containers, func(container corev1.Container) bool {
		return container.Name == sidecar.Name
	})

	if sidecarContainerFound {
		// The sidecar container was already added
		return nil
	}

	if !postgresContainerFound {
		return ErrNoPostgresContainerFound
	}

	// Do not modify the passed sidecar definition
	if injectPostgresVolumeMounts {
		sidecar.VolumeMounts = append(sidecar.VolumeMounts, volumeMounts...)
	}
	containers = append(containers, *sidecar)

	*targetContainerList = containers

	return nil
}
