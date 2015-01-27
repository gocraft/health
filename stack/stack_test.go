package stack

import (
	"fmt"
	"regexp"
	"testing"
	// "github.com/stretchr/testify/assert"
)

func level2() *Trace {
	return NewTrace(0)
}

func level1() *Trace {
	return level2()
}

func level0() *Trace {
	return level1()
}

func assertFrame(t *testing.T, frame *Frame, file string, line int, fun string) {
	testName := fmt.Sprintf("[file: %s line: %d fun: %s]", file, line, fun)

	if !regexp.MustCompile(file).MatchString(frame.File) {
		t.Errorf("assertFrame: %s didn't match file in %v", testName, frame)
	}

	if frame.LineNumber != line {
		t.Errorf("assertFrame: %s didn't match line in %v", testName, frame)
	}

	if frame.Name != fun {
		t.Errorf("assertFrame: %s didn't match function name in %v", testName, frame)
	}
}

func TestNewTrace(t *testing.T) {
	trace := level0()

	frames := trace.Frames()

	// Yes, this is a persnickety test that will fail as the file is modified. Sorry guise.
	assertFrame(t, &frames[0], "stack_test\\.go", 11, "level2")
	assertFrame(t, &frames[1], "stack_test\\.go", 15, "level1")
	assertFrame(t, &frames[2], "stack_test\\.go", 19, "level0")
	assertFrame(t, &frames[3], "stack_test\\.go", 39, "TestNewTrace")
}

type someT struct{}

func (s someT) level2() *Trace {
	return NewTrace(0)
}

func (s someT) level1() *Trace {
	return s.level2()
}

func (s someT) level0() *Trace {
	return s.level1()
}

func TestNewTraceWithTypes(t *testing.T) {
	obj := &someT{}
	trace := obj.level0()

	frames := trace.Frames()

	// Yes, this is a persnickety test that will fail as the file is modified. Sorry guise.
	assertFrame(t, &frames[0], "stack_test\\.go", 53, "someT.level2")
	assertFrame(t, &frames[1], "stack_test\\.go", 57, "someT.level1")
	assertFrame(t, &frames[2], "stack_test\\.go", 61, "someT.level0")
	assertFrame(t, &frames[3], "stack_test\\.go", 66, "TestNewTraceWithTypes")
}

func TestStackPrint(t *testing.T) {
	trace := level0()
	stack := trace.Stack()
	reg := regexp.MustCompile("stack_test\\.go:11 level2\n.+stack_test\\.go:15 level1\n.+stack_test\\.go:19 level0")

	if !reg.Match(trace.Stack()) {
		t.Errorf("trace didn't match. Got:\n%s\n", string(stack))
	}
}
