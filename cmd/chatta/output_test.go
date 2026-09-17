package main

import (
	"bytes"
	"testing"

	"github.com/alswl/chatta/pkg/common"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// `null` is the one output shape that breaks a consumer instead of merely
// disappointing it: `jq '.[]'` iterates an empty array quietly and errors on
// null, which buries the real reason -- on stderr -- under a type complaint.
// A failing command hands back the nil slice from its error path, so this is
// the case that matters.
func TestEmitJSONListNeverWritesNull(t *testing.T) {
	for _, tc := range []struct {
		name string
		emit func(*cobra.Command) error
	}{
		{"nil messages", func(c *cobra.Command) error { return emitJSONList(c, []common.StoredMessage(nil)) }},
		{"nil clients", func(c *cobra.Command) error { return emitJSONList(c, []common.ClientSurvey(nil)) }},
		{"empty clients", func(c *cobra.Command) error { return emitJSONList(c, []common.ClientSurvey{}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&out)
			require.NoError(t, tc.emit(cmd))
			require.Equal(t, "[]\n", out.String())
		})
	}
}

func TestEmitJSONListKeepsContent(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	require.NoError(t, emitJSONList(cmd, []common.ChannelMember{{Nick: "misky", You: true}}))
	require.JSONEq(t, `[{"nick":"misky","you":true}]`, out.String())
}
