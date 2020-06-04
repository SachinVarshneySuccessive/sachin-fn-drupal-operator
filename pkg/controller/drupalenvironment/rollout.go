package drupalenvironment

import (
	rolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/argoproj/argo-rollouts/utils/conditions"
	corev1 "k8s.io/api/core/v1"
)

func isNewRSAvailable(rollout *rolloutsv1alpha1.Rollout) bool {
	progressingCondition := conditions.GetRolloutCondition(rollout.Status, rolloutsv1alpha1.RolloutProgressing)
	return progressingCondition != nil &&
		progressingCondition.Status == corev1.ConditionTrue &&
		progressingCondition.Reason == conditions.NewRSAvailableReason
}

func isReplicaSetUpdated(rollout *rolloutsv1alpha1.Rollout) bool {
	progressingCondition := conditions.GetRolloutCondition(rollout.Status, rolloutsv1alpha1.RolloutProgressing)
	return progressingCondition != nil &&
		progressingCondition.Status == corev1.ConditionTrue &&
		progressingCondition.Reason == conditions.ReplicaSetUpdatedReason
}

func isAvailable(rollout *rolloutsv1alpha1.Rollout) bool {
	availableCondition := conditions.GetRolloutCondition(rollout.Status, rolloutsv1alpha1.RolloutAvailable)
	return availableCondition != nil &&
		availableCondition.Status == corev1.ConditionTrue &&
		availableCondition.Reason == conditions.AvailableReason
}
