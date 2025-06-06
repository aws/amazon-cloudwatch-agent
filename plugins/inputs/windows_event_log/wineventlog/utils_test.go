// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package wineventlog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateFilterQuery(t *testing.T) {
	tests := []struct {
		name     string
		levels   []string
		eventIDs []int
		want     string
	}{
		{
			name:     "levels_Test",
			levels:   []string{"Error", "Critical"},
			eventIDs: []int{},
			want:     "*[System[(Level='Error' or Level='Critical') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]",
		},
		{
			name:     "EventID_Test",
			levels:   []string{},
			eventIDs: []int{1001, 1002},
			want:     "*[System[(EventID='1001' or EventID='1002') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]",
		},
		{
			name:     "levels_EventID_Test",
			levels:   []string{"Error", "Critical"},
			eventIDs: []int{4625, 4624},
			want:     "*[System[(Level='Error' or Level='Critical') and (EventID='4625' or EventID='4624') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]",
		},
		{
			name:     "no_Input",
			levels:   []string{},
			eventIDs: []int{},
			want:     "*[System[TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createFilterQuery(tt.levels, tt.eventIDs)
			assert.Equal(t, tt.want, got)

		})
	}
}
