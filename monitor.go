package cachet

import (
	"sync"
	"time"
	"strconv"
	"os/exec"

	"github.com/Sirupsen/logrus"
)

const DefaultInterval = time.Second * 60
const DefaultTimeout = time.Second
const DefaultTimeFormat = "15:04:05 Jan 2 MST"
const HistorySize = 10

type MonitorInterface interface {
	ClockStart(*CachetMonitor, MonitorInterface, *sync.WaitGroup)
	ClockStop()
	tick(MonitorInterface)
	test(l *logrus.Entry) bool

	Init(*CachetMonitor)
	Validate() []string
	GetMonitor() *AbstractMonitor
	Describe() []string
}

// AbstractMonitor data model
type AbstractMonitor struct {
	Name   string
	Target string

	// (default)http / dns
	Type   string
	Strict bool

	Interval time.Duration
	Timeout  time.Duration

	MetricID    int `mapstructure:"metric_id"`
	ComponentID int `mapstructure:"component_id"`

	// Metric stuff
	Metrics struct {
		ResponseTime []int	`mapstructure:"response_time"`
		Availability []int	`mapstructure:"availability"`
		IncidentCount []int	`mapstructure:"incident_count"`
	}

	// ShellHook stuff
	ShellHook struct {
		OnSuccess string	`mapstructure:"on_success"`
		OnFailure string	`mapstructure:"on_failure"`
	}

	// Templating stuff
	Template struct {
		Investigating MessageTemplate
		Fixed         MessageTemplate
	}

	// Threshold = percentage / number of down incidents
	Threshold      float32
	ThresholdCount bool `mapstructure:"threshold_count"`

	CriticalThreshold      float32 `mapstructure:"threshold_critical"`
	CriticalThresholdCount bool `mapstructure:"threshold_critical_count"`

	// lag / average(lagHistory) * 100 = percentage above average lag
	// PerformanceThreshold sets the % limit above which this monitor will trigger degraded-performance
	// PerformanceThreshold float32

	currentStatus	int
	history []bool
	// lagHistory     []float32
	lastFailReason string
	incident       *Incident
	config         *CachetMonitor

	// Closed when mon.Stop() is called
	stopC chan bool
}

func (mon *AbstractMonitor) Validate() []string {
	errs := []string{}

	if len(mon.Name) == 0 {
		errs = append(errs, "Name is required")
	}

	if mon.Interval < 1 {
		mon.Interval = DefaultInterval
	}
	if mon.Timeout < 1 {
		mon.Timeout = DefaultTimeout
	}

	if mon.Timeout > mon.Interval {
		errs = append(errs, "Timeout greater than interval")
	}

	if mon.ComponentID == 0 && mon.MetricID == 0 {
		errs = append(errs, "component_id & metric_id are unset")
	}

	if mon.Threshold <= 0 {
		mon.Threshold = 100
	}

	if err := mon.Template.Fixed.Compile(); err != nil {
		errs = append(errs, "Could not compile \"fixed\" template: "+err.Error())
	}
	if err := mon.Template.Investigating.Compile(); err != nil {
		errs = append(errs, "Could not compile \"investigating\" template: "+err.Error())
	}

	return errs
}
func (mon *AbstractMonitor) GetMonitor() *AbstractMonitor {
	return mon
}
func (mon *AbstractMonitor) Describe() []string {
	features := []string{"Type: " + mon.Type}

	if len(mon.Name) > 0 {
		features = append(features, "Name: "+mon.Name)
	}
	features = append(features, "Availability count metrics: "+strconv.Itoa(len(mon.Metrics.Availability)))
	features = append(features, "Incident count metrics: "+strconv.Itoa(len(mon.Metrics.IncidentCount)))
	features = append(features, "Response time metrics: "+strconv.Itoa(len(mon.Metrics.ResponseTime)))
	if len(mon.ShellHook.OnSuccess) > 0 {
		features = append(features, "Has a 'on_success' shellhook")
	}
	if len(mon.ShellHook.OnFailure) > 0 {
		features = append(features, "Has a 'on_failure' shellhook")
	}

	return features
}

func (mon *AbstractMonitor) Init(cfg *CachetMonitor) {
	mon.config = cfg

	compInfo := mon.config.API.GetComponentData(mon.ComponentID)

	logrus.Infof("Current CachetHQ ID: %d", compInfo.ID)
	logrus.Infof("Current CachetHQ name: %s", compInfo.Name)
	logrus.Infof("Current CachetHQ status: %d", compInfo.Status)

	mon.currentStatus = compInfo.Status
	mon.history = append(mon.history, mon.isUp())
	if ! mon.isUp() {
		mon.incident,_ = compInfo.LoadCurrentIncident(cfg)
		if mon.incident != nil {
			logrus.Infof("Current incident ID: %v", mon.incident.ID)
		}
	}
}

func (mon *AbstractMonitor) triggerShellHook(l *logrus.Entry, hooktype string, hook string, data string) {
	if len(hook) == 0 {
		return
	}
	l.Infof("Sending '%s' shellhook", hooktype)
	l.Debugf("Data: %s", data)

	out, err := exec.Command(hook, mon.Name, strconv.Itoa(mon.ComponentID), mon.Target, hooktype, data).Output()
	if err != nil {
	    l.Warnf("Error when processing shellhook '%s': %s", hooktype, err)
	    l.Warnf("Command output: %s", out)
	}
}

func (mon *AbstractMonitor) ClockStart(cfg *CachetMonitor, iface MonitorInterface, wg *sync.WaitGroup) {
	wg.Add(1)

	mon.stopC = make(chan bool)
	if cfg.Immediate {
		mon.tick(iface)
	}

	ticker := time.NewTicker(mon.Interval * time.Second)
	for {
		select {
		case <-ticker.C:
			mon.tick(iface)
		case <-mon.stopC:
			wg.Done()
			return
		}
	}
}

func (mon *AbstractMonitor) ClockStop() {
	select {
	case <-mon.stopC:
		return
	default:
		close(mon.stopC)
	}
}

func (mon *AbstractMonitor) isUp() bool {
	return (mon.currentStatus == 1)
}

func (mon *AbstractMonitor) test(l *logrus.Entry) bool { return false }

func (mon *AbstractMonitor) tick(iface MonitorInterface) {
	l := logrus.WithFields(logrus.Fields{
		"monitor": mon.Name })

	reqStart := getMs()
	isUp := iface.test(l)
	lag := getMs() - reqStart

	histSize := HistorySize
	if len(mon.history) == histSize-1 {
		l.Debugf("monitor %v is now fully operational", mon.Name)
	}
	if mon.ThresholdCount {
		histSize = int(mon.Threshold)
	}

	if len(mon.history) >= histSize {
		mon.history = mon.history[len(mon.history)-(histSize-1):]
	}
	mon.history = append(mon.history, isUp)

	mon.AnalyseData(l)

	// Will trigger shellhook 'on_failure' as this isn't done in implementations
	if ! isUp {
		mon.triggerShellHook(l, "on_failure", mon.ShellHook.OnFailure, "")
	}

	// report lag
	if mon.MetricID > 0 {
		go mon.config.API.SendMetric(l, mon.MetricID, lag)
	}
	go mon.config.API.SendMetrics(l, "response time", mon.Metrics.ResponseTime, lag)
}

// TODO: test
// AnalyseData decides if the monitor is statistically up or down and creates / resolves an incident
func (mon *AbstractMonitor) AnalyseData(l *logrus.Entry) {
	// look at the past few incidents
	numDown := 0
	for _, wasUp := range mon.history {
		if wasUp == false {
			numDown++
		}
	}

	t := (float32(numDown) / float32(len(mon.history))) * 100
	l.Debugf("Down count: %d, history: %d, percentage: %.2f", numDown, len(mon.history), t)
	l.Debugf("Threshold: %d", int(mon.Threshold))
	if numDown == 0 {
		l.Printf("monitor is up")
		go mon.config.API.SendMetrics(l, "availability", mon.Metrics.Availability, 1)
	} else if mon.ThresholdCount {
		l.Printf("monitor down (down count=%d, threshold=%d)", t, mon.Threshold)
	} else {
		l.Printf("monitor down (down percentage=%.2f%%, threshold=%.2f%%)", t, mon.Threshold)
	}

	histSize := HistorySize
	if mon.ThresholdCount {
		histSize = int(mon.Threshold)
	}

	if len(mon.history) != histSize {
		// not yet saturated
		l.Debugf("Component's history has not been yet saturated (stack: %d/%d)", len(mon.history), histSize)
		return
	}

	triggered := false
	if mon.ThresholdCount || mon.Threshold > 0 {
		triggered = (mon.ThresholdCount && numDown == int(mon.Threshold)) || (!mon.ThresholdCount && t > mon.Threshold)
	}
	
	criticalTriggered := false
	if mon.CriticalThresholdCount || mon.CriticalThreshold > 0 {
		criticalTriggered = (mon.CriticalThresholdCount && numDown == int(mon.CriticalThreshold)) || (!mon.CriticalThresholdCount && t > mon.CriticalThreshold)
	}

	l.Debugf("Down counter: %d", numDown)
	l.Debugf("Down percentage: %f", t)
	l.Debugf("Triggered: %b", triggered)
	l.Debugf("Critically Triggered: %b", criticalTriggered)
	l.Debugf("Monitor's current incident: %v", mon.incident)

	if triggered {
		// Process metric
		go mon.config.API.SendMetrics(l, "incident count", mon.Metrics.IncidentCount, 1)

		if mon.incident == nil {
			// create incident
			mon.currentStatus = 2
			tplData := getTemplateData(mon)
			tplData["FailReason"] = mon.lastFailReason

			subject, message := mon.Template.Investigating.Exec(tplData)
			mon.incident = &Incident{
				Name:        subject,
				ComponentID: mon.ComponentID,
				Message:     message,
				Notify:      true,
			}

			// is down, create an incident
			l.Warnf("creating incident. Monitor is down: %v", mon.lastFailReason)
			// set investigating status
			mon.incident.SetInvestigating()
			// create/update incident
			if err := mon.incident.Send(mon.config); err != nil {
				l.Printf("Error sending incident: %v", err)
			}
		}
		if criticalTriggered {
			if (mon.currentStatus != 4) {
				mon.config.API.SetComponentStatus(mon, 4)
			}
		}
		return
	}

	// we are up to normal

	// global status seems incorrect though we couldn't fid any prior incident
	if ! mon.isUp() && mon.incident == nil {
		l.Info("Reseting component's status")
		mon.lastFailReason = ""
		mon.incident = nil
		mon.config.API.SetComponentStatus(mon, 1)
		return
	}

	if mon.incident == nil {
		return
	}

	// was down, created an incident, its now ok, make it resolved.
	l.Warn("Resolving incident")

	// resolve incident
	tplData := getTemplateData(mon)
	tplData["incident"] = mon.incident

	subject, message := mon.Template.Fixed.Exec(tplData)
	mon.incident.Name = subject
	mon.incident.Message = message
	mon.incident.SetFixed()
	if err := mon.incident.Send(mon.config); err != nil {
		l.Printf("Error sending incident: %v", err)
	}

	mon.lastFailReason = ""
	mon.incident = nil
	mon.currentStatus = 1
}
