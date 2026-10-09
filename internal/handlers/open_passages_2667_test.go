package handlers

import (
	"context"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"systemburo/internal/services"
	"testing"
)

type passageReader2667 struct {
	actor  int
	kind   services.ElementKind
	table  *int
	filter services.PassageOpenFilter
	calls  int
}

func (r *passageReader2667) List(_ context.Context, actor int, kind services.ElementKind, table *int, filter services.PassageOpenFilter) (*services.PassageOpenList, error) {
	r.actor = actor
	r.kind = kind
	r.table = table
	r.filter = filter
	r.calls++
	return &services.PassageOpenList{Items: []services.PassageOpenItem{}}, nil
}

type passageCommands2667 struct {
	actor  int
	id     int
	req    services.PassageCorrectionRequest
	revert bool
}

func (s *passageCommands2667) Correct(_ context.Context, actor int, _ services.ElementKind, id int, req services.PassageCorrectionRequest) (*services.PassageResult, error) {
	s.actor = actor
	s.id = id
	s.req = req
	return &services.PassageResult{}, nil
}
func (s *passageCommands2667) RevertCorrection(ctx context.Context, actor int, kind services.ElementKind, id int, req services.PassageCorrectionRequest) (*services.PassageResult, error) {
	s.revert = true
	return s.Correct(ctx, actor, kind, id, req)
}

func TestOpenPassages2667HandlerList(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		summary     bool
		attention   bool
		invalid     bool
	}{
		{name: "default", attention: true},
		{name: "all", query: "?attention_only=false", attention: false},
		{name: "corrections", query: "?view=corrections", attention: false},
		{name: "summary", summary: true, attention: true},
		{name: "invalid page", query: "?page=abc", invalid: true},
		{name: "invalid flag", query: "?attention_only=maybe", invalid: true},
		{name: "invalid organization", query: "?organization_id=-2", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/"+tc.query, nil), httptest.NewRecorder())
			c.Set("user_id", 7)
			c.SetParamNames("table_id")
			c.SetParamValues("2")
			reader := &passageReader2667{}
			h := NewOpenPassagesHandler(reader, nil)
			err := h.list(c, services.ElementEmployee, tc.summary)
			if tc.invalid {
				require.Error(t, err)
				require.Zero(t, reader.calls)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 7, reader.actor)
			require.Equal(t, tc.attention, reader.filter.AttentionOnly)
			if tc.summary {
				require.Nil(t, reader.table)
			} else {
				require.Equal(t, 2, *reader.table)
			}
		})
	}
}
func TestOpenPassages2667HandlerActorComesFromContext(t *testing.T) {
	e := echo.New()
	body := `{"actor_user_id":999,"user_id":999,"source":"table","table_id":2,"expected_last_event_id":12,"reason":"Synthetic correction"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := e.NewContext(req, httptest.NewRecorder())
	c.Set("user_id", 7)
	c.SetParamNames("id")
	c.SetParamValues("3")
	commands := &passageCommands2667{}
	h := NewOpenPassagesHandler(nil, commands)
	require.NoError(t, h.CloseCar(c))
	require.Equal(t, 7, commands.actor)
	require.Equal(t, 3, commands.id)
	require.EqualValues(t, 12, *commands.req.ExpectedLastEventID)
	c.Set("user_id", 0)
	require.Error(t, h.CloseCar(c))
}
