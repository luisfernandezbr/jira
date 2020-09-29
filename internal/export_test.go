package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const missingProjectErr = "{\"errorMessages\":[\"A value with ID '10600' does not exist for the field 'project'.\",\"A value with ID '13017' does not exist for the field 'project'.\",\"A value with ID '13064' does not exist for the field 'project'.\",\"A value with ID '13069' does not exist for the field 'project'.\",\"A value with ID '13070' does not exist for the field 'project'.\",\"A value with ID '13072' does not exist for the field 'project'.\",\"A value with ID '13077' does not exist for the field 'project'.\",\"A value with ID '13085' does not exist for the field 'project'.\",\"A value with ID '13116' does not exist for the field 'project'.\"],\"warningMessages\":[]}"

func TestGetInvalidProjects(t *testing.T) {
	assert := assert.New(t)
	e, ok := toIssueError([]byte(missingProjectErr))
	assert.True(ok)
	v := e.getInvalidProjects()
	assert.Len(v, 9)
	assert.EqualValues("10600", v[0])
	assert.EqualValues("13017", v[1])
	assert.EqualValues("13064", v[2])
	assert.EqualValues("13069", v[3])
	assert.EqualValues("13070", v[4])
	assert.EqualValues("13072", v[5])
	assert.EqualValues("13077", v[6])
	assert.EqualValues("13085", v[7])
	assert.EqualValues("13116", v[8])
}
