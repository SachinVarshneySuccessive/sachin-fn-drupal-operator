package common

const WorkerNodeLabel = "node-role.kubernetes.io/worker"
const ManagementNodeLabel = "node-role.kubernetes.io/management"
const IngressNodeLabel = "node-role.kubernetes.io/ingress"

type Labels map[string]string

// MergeLabels returns the union of two sets of Kubernetes Labels, with values in "overlay" taking precedence when
// collisions occur.
func MergeLabels(base, overlay Labels) Labels {
	if base == nil {
		base = make(map[string]string, len(overlay))
	}

	for k, v := range overlay {
		base[k] = v
	}

	if len(base) == 0 {
		return nil
	}
	return base
}

// MergeAnnotations returns the union of two sets of Kubernetes Annotations, with values in "overlay" taking precedence
// when collisions occur.
func MergeAnnotations(base, overlay Labels) Labels {
	return MergeLabels(base, overlay) // Since Annotations are just key-value string pairs, too.
}
