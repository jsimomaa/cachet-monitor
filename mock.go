package cachet

import (
	"github.com/Sirupsen/logrus"
)

type MockMonitor struct {
	AbstractMonitor `mapstructure:",squash"`
}

// TODO: test
func (monitor *MockMonitor) test(l *logrus.Entry) bool {
	monitor.triggerShellHook(l, "on_success", monitor.ShellHook.OnSuccess, "")

	return true
}

// TODO: test
func (mon *MockMonitor) Validate() []string {

	errs := mon.AbstractMonitor.Validate()

	return errs
}

func (mon *MockMonitor) Describe() []string {
	features := mon.AbstractMonitor.Describe()

	return features
}
