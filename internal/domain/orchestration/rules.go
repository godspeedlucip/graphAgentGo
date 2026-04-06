package orchestration

func NormalizeMaxSteps(v int) int {
	if v <= 0 {
		return 8
	}
	return v
}

func ShouldStop(st GraphState) bool {
	if st.Stopped {
		return true
	}
	if st.StepIndex >= st.MaxSteps {
		return true
	}
	return false
}
