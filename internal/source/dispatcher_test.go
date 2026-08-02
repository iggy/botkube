package source

import (
	"testing"

	"github.com/iggy/botkube/pkg/loggerx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingObserver struct {
	pluginNames []string
	rawObjects  []any
	panics      bool
}

func (o *recordingObserver) ObserveEvent(pluginName string, rawObject any) {
	o.pluginNames = append(o.pluginNames, pluginName)
	o.rawObjects = append(o.rawObjects, rawObject)
	if o.panics {
		panic("boom")
	}
}

func TestObserveEventWithoutObserver(t *testing.T) {
	// The observer is optional, so a dispatcher without one must behave exactly as before.
	dispatcher := NewDispatcher(loggerx.NewNoop(), "test", nil, nil, nil, nil, nil, nil, nil)

	assert.NotPanics(t, func() {
		dispatcher.observeEvent("botkube/kubernetes", map[string]any{"Kind": "Pod"})
	})
}

func TestObserveEventForwardsToObserver(t *testing.T) {
	// given
	observer := &recordingObserver{}
	dispatcher := NewDispatcher(loggerx.NewNoop(), "test", nil, nil, nil, nil, nil, nil, nil).WithStateObserver(observer)
	event := map[string]any{"Kind": "Pod", "Name": "api-1"}

	// when
	dispatcher.observeEvent("botkube/kubernetes", event)

	// then
	require.Equal(t, []string{"botkube/kubernetes"}, observer.pluginNames)
	assert.Equal(t, []any{event}, observer.rawObjects)
}

func TestObserveEventSurvivesPanickingObserver(t *testing.T) {
	// The observer maintains an accessory view of the cluster. A panic in it must be contained here,
	// because the caller goes on to deliver the event to every notifier.
	observer := &recordingObserver{panics: true}
	dispatcher := NewDispatcher(loggerx.NewNoop(), "test", nil, nil, nil, nil, nil, nil, nil).WithStateObserver(observer)

	assert.NotPanics(t, func() {
		dispatcher.observeEvent("botkube/kubernetes", map[string]any{"Kind": "Pod"})
	})
	assert.Len(t, observer.pluginNames, 1)
}
