package radix

import (
	crand "crypto/rand"
	"fmt"
	"sort"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestRadix(t *testing.T) {
	c := qt.New(t)
	var min, max string
	inp := make(map[string]int)
	for i := range 1000 {
		gen := generateUUID()
		inp[gen] = i
		if gen < min || i == 0 {
			min = gen
		}
		if gen > max || i == 0 {
			max = gen
		}
	}

	r := NewFromMap(inp)
	c.Assert(r.Len(), qt.Equals, len(inp))

	r.Walk(func(k string, v int) bool {
		return false
	})

	for k, v := range inp {
		out, ok := r.Get(k)
		c.Assert(ok, qt.IsTrue)
		c.Assert(out, qt.Equals, v)
	}

	// Check min and max
	outMin, _, _ := r.Minimum()
	c.Assert(outMin, qt.Equals, min)
	outMax, _, _ := r.Maximum()
	c.Assert(outMax, qt.Equals, max)

	for k, v := range inp {
		out, ok := r.Delete(k)
		c.Assert(ok, qt.IsTrue)
		c.Assert(out, qt.Equals, v)
	}
	c.Assert(r.Len(), qt.Equals, 0)
}

func TestRoot(t *testing.T) {
	c := qt.New(t)
	r := New[bool]()
	_, ok := r.Delete("")
	c.Assert(ok, qt.IsFalse)
	_, ok = r.Insert("", true)
	c.Assert(ok, qt.IsFalse)
	val, ok := r.Get("")
	c.Assert(ok, qt.IsTrue)
	c.Assert(val, qt.IsTrue)
	val, ok = r.Delete("")
	c.Assert(ok, qt.IsTrue)
	c.Assert(val, qt.IsTrue)
}

func TestDelete(t *testing.T) {
	c := qt.New(t)
	r := New[bool]()

	s := []string{"", "A", "AB"}

	for _, ss := range s {
		r.Insert(ss, true)
	}

	for _, ss := range s {
		_, ok := r.Delete(ss)
		c.Assert(ok, qt.IsTrue)
	}
}

func TestDeletePrefix(t *testing.T) {
	c := qt.New(t)
	type exp struct {
		inp        []string
		prefix     string
		out        []string
		numDeleted int
	}

	cases := []exp{
		{[]string{"", "A", "AB", "ABC", "R", "S"}, "A", []string{"", "R", "S"}, 3},
		{[]string{"", "A", "AB", "ABC", "R", "S"}, "ABC", []string{"", "A", "AB", "R", "S"}, 1},
		{[]string{"", "A", "AB", "ABC", "R", "S"}, "", []string{}, 6},
		{[]string{"", "A", "AB", "ABC", "R", "S"}, "S", []string{"", "A", "AB", "ABC", "R"}, 1},
		{[]string{"", "A", "AB", "ABC", "R", "S"}, "SS", []string{"", "A", "AB", "ABC", "R", "S"}, 0},
	}

	for _, test := range cases {
		r := New[bool]()
		for _, ss := range test.inp {
			r.Insert(ss, true)
		}

		deleted := r.DeletePrefix(test.prefix)
		c.Assert(deleted, qt.Equals, test.numDeleted)

		out := []string{}
		fn := func(s string, v bool) bool {
			out = append(out, s)
			return false
		}
		r.Walk(fn)

		c.Assert(out, qt.DeepEquals, test.out)
	}
}

func TestLongestPrefix(t *testing.T) {
	c := qt.New(t)
	r := New[any]()

	keys := []string{
		"",
		"foo",
		"foobar",
		"foobarbaz",
		"foobarbazzip",
		"foozip",
	}
	for _, k := range keys {
		r.Insert(k, nil)
	}
	c.Assert(r.Len(), qt.Equals, len(keys))

	type exp struct {
		inp string
		out string
	}
	cases := []exp{
		{"a", ""},
		{"abc", ""},
		{"fo", ""},
		{"foo", "foo"},
		{"foob", "foo"},
		{"foobar", "foobar"},
		{"foobarba", "foobar"},
		{"foobarbaz", "foobarbaz"},
		{"foobarbazzi", "foobarbaz"},
		{"foobarbazzip", "foobarbazzip"},
		{"foozi", "foo"},
		{"foozip", "foozip"},
		{"foozipzap", "foozip"},
	}
	for _, test := range cases {
		m, _, ok := r.LongestPrefix(test.inp)
		if test.out != "" {
			c.Assert(ok, qt.IsTrue)
		}
		c.Assert(m, qt.Equals, test.out)
	}
}

func TestWalkPrefix(t *testing.T) {
	c := qt.New(t)
	r := New[any]()

	keys := []string{
		"foobar",
		"foo/bar/baz",
		"foo/baz/bar",
		"foo/zip/zap",
		"zipzap",
	}
	for _, k := range keys {
		r.Insert(k, nil)
	}
	c.Assert(r.Len(), qt.Equals, len(keys))

	type exp struct {
		inp string
		out []string
	}
	cases := []exp{
		{
			"f",
			[]string{"foobar", "foo/bar/baz", "foo/baz/bar", "foo/zip/zap"},
		},
		{
			"foo",
			[]string{"foobar", "foo/bar/baz", "foo/baz/bar", "foo/zip/zap"},
		},
		{
			"foob",
			[]string{"foobar"},
		},
		{
			"foo/",
			[]string{"foo/bar/baz", "foo/baz/bar", "foo/zip/zap"},
		},
		{
			"foo/b",
			[]string{"foo/bar/baz", "foo/baz/bar"},
		},
		{
			"foo/ba",
			[]string{"foo/bar/baz", "foo/baz/bar"},
		},
		{
			"foo/bar",
			[]string{"foo/bar/baz"},
		},
		{
			"foo/bar/baz",
			[]string{"foo/bar/baz"},
		},
		{
			"foo/bar/bazoo",
			[]string{},
		},
		{
			"z",
			[]string{"zipzap"},
		},
	}

	for _, test := range cases {
		out := []string{}
		fn := func(s string, v any) bool {
			out = append(out, s)
			return false
		}
		r.WalkPrefix(test.inp, fn)
		sort.Strings(out)
		sort.Strings(test.out)
		c.Assert(out, qt.DeepEquals, test.out)
	}
}

func TestWalkPath(t *testing.T) {
	c := qt.New(t)
	r := New[any]()

	keys := []string{
		"foo",
		"foo/bar",
		"foo/bar/baz",
		"foo/baz/bar",
		"foo/zip/zap",
		"zipzap",
	}
	for _, k := range keys {
		r.Insert(k, nil)
	}
	c.Assert(r.Len(), qt.Equals, len(keys))

	type exp struct {
		inp string
		out []string
	}
	cases := []exp{
		{
			"f",
			[]string{},
		},
		{
			"foo",
			[]string{"foo"},
		},
		{
			"foo/",
			[]string{"foo"},
		},
		{
			"foo/ba",
			[]string{"foo"},
		},
		{
			"foo/bar",
			[]string{"foo", "foo/bar"},
		},
		{
			"foo/bar/baz",
			[]string{"foo", "foo/bar", "foo/bar/baz"},
		},
		{
			"foo/bar/bazoo",
			[]string{"foo", "foo/bar", "foo/bar/baz"},
		},
		{
			"z",
			[]string{},
		},
	}

	for _, test := range cases {
		out := []string{}
		fn := func(s string, v any) bool {
			out = append(out, s)
			return false
		}
		r.WalkPath(test.inp, fn)
		sort.Strings(out)
		sort.Strings(test.out)
		c.Assert(out, qt.DeepEquals, test.out)
	}
}

func TestWalkDelete(t *testing.T) {
	c := qt.New(t)
	r := New[any]()
	r.Insert("init0/0", nil)
	r.Insert("init0/1", nil)
	r.Insert("init0/2", nil)
	r.Insert("init0/3", nil)
	r.Insert("init1/0", nil)
	r.Insert("init1/1", nil)
	r.Insert("init1/2", nil)
	r.Insert("init1/3", nil)
	r.Insert("init2", nil)

	deleteFn := func(s string, v any) bool {
		r.Delete(s)
		return false
	}

	r.WalkPrefix("init1", deleteFn)

	for _, s := range []string{"init0/0", "init0/1", "init0/2", "init0/3", "init2"} {
		_, ok := r.Get(s)
		c.Assert(ok, qt.IsTrue)
	}
	c.Assert(r.Len(), qt.Equals, 5)

	r.Walk(deleteFn)
	c.Assert(r.Len(), qt.Equals, 0)
}

// generateUUID is used to generate a random UUID
func generateUUID() string {
	buf := make([]byte, 16)
	if _, err := crand.Read(buf); err != nil {
		panic(fmt.Errorf("failed to read random bytes: %v", err))
	}

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])
}

func BenchmarkInsert(b *testing.B) {
	c := qt.New(b)
	r := New[bool]()
	for i := range 10000 {
		r.Insert(fmt.Sprintf("init%d", i), true)
	}

	b.ResetTimer()

	for n := 0; b.Loop(); n++ {
		_, updated := r.Insert(strconv.Itoa(n), true)
		c.Assert(updated, qt.IsFalse)
	}
}

func BenchmarkRadix(b *testing.B) {
	c := qt.New(b)
	type v struct {
		s string
	}
	r := New[*v]()

	for i := range 100 {
		for j := range 100 {
			r.Insert(fmt.Sprintf("init%d/%d", i, j), &v{s: "hello"})
		}
	}

	b.ResetTimer()

	b.Run("Walk", func(b *testing.B) {
		for b.Loop() {
			r.Walk(func(s string, v *v) bool {
				return false
			})
		}
	})

	b.Run("WalkPrefix", func(b *testing.B) {
		for b.Loop() {
			r.WalkPrefix("init50", func(s string, v *v) bool {
				return false
			})
		}
	})

	b.Run("WalkPath", func(b *testing.B) {
		for b.Loop() {
			r.WalkPath("init50/50", func(s string, v *v) bool {
				return false
			})
		}
	})

	b.Run("Get", func(b *testing.B) {
		for b.Loop() {
			v, ok := r.Get("init50/50")
			_ = v
			c.Assert(ok, qt.IsTrue)
		}
	})

	b.Run("LongestPrefix", func(b *testing.B) {
		for b.Loop() {
			s, v, ok := r.LongestPrefix("init50/50")
			_ = s
			_ = v
			c.Assert(ok, qt.IsTrue)
		}
	})
}
