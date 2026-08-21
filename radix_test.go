package radix

import (
	crand "crypto/rand"
	"fmt"
	"sort"
	"strconv"
	"sync"
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

	var fn WalkFn[int] = func(s string, v int) (WalkFlag, int, error) {
		return WalkContinue, v, nil
	}
	r.Walk(fn)

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

func TestWalkSet(t *testing.T) {
	c := qt.New(t)

	newTree := func() *Tree[int] {
		r := New[int]()

		for i := range 10 {
			r.Insert(fmt.Sprintf("key%d", i), i)
		}
		return r
	}

	collect := func(r *Tree[int]) []int {
		var ints []int
		var fn WalkFn[int] = func(s string, v int) (WalkFlag, int, error) {
			ints = append(ints, v)
			return WalkContinue, 0, nil
		}
		r.Walk(fn)
		return ints
	}

	c.Run("Basic", func(c *qt.C) {
		var fn WalkFn[int] = func(s string, v int) (WalkFlag, int, error) {
			k := fmt.Sprintf("key%d", v)
			c.Assert(s, qt.Equals, k)
			v2 := v
			if v%2 == 0 {
				v2 = v * 10
				return WalkSet, v2, nil
			}
			return WalkContinue, 0, nil
		}

		r := newTree()
		r.Walk(fn)

		c.Assert(collect(r), qt.DeepEquals, []int{0, 1, 20, 3, 40, 5, 60, 7, 80, 9})
	})

	c.Run("Skip some ", func(c *qt.C) {
		r := newTree()
		h := testWalkHandler[int]{
			check: func(s string) (WalkFlag, error) {
				if s == "key4" || s == "key7" {
					return WalkSkip, nil
				}
				return WalkContinue, nil
			},
			handle: func(s string, v int) (WalkFlag, int, error) {
				return WalkSet, v * 10, nil
			},
		}
		r.Walk(h)

		c.Assert(collect(r), qt.DeepEquals, []int{0, 10, 20, 30, 4, 50, 60, 7, 80, 90})
	})

	c.Run("Stop in check", func(c *qt.C) {
		r := newTree()
		h := testWalkHandler[int]{
			check: func(s string) (WalkFlag, error) {
				if s == "key4" {
					return WalkStop, nil
				}
				return WalkContinue, nil
			},
			handle: func(s string, v int) (WalkFlag, int, error) {
				return WalkSet, v * 10, nil
			},
		}
		r.Walk(h)

		c.Assert(collect(r), qt.DeepEquals, []int{0, 10, 20, 30, 4, 5, 6, 7, 8, 9})
	})

	c.Run("Stop in handle", func(c *qt.C) {
		r := newTree()
		h := testWalkHandler[int]{
			check: func(s string) (WalkFlag, error) {
				return WalkContinue, nil
			},
			handle: func(s string, v int) (WalkFlag, int, error) {
				if s == "key4" {
					return WalkStop, 0, nil
				}
				return WalkSet, v * 10, nil
			},
		}
		r.Walk(h)

		c.Assert(collect(r), qt.DeepEquals, []int{0, 10, 20, 30, 4, 5, 6, 7, 8, 9})
	})
}

func TestWalkSetParallel(t *testing.T) {
	c := qt.New(t)

	r := New[int]()

	for i := range 1000 {
		r.Insert(fmt.Sprintf("key%d", i), i)
	}

	var wg sync.WaitGroup

	for i := range 10 {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			if j == 2 {
				var fn WalkFn[int] = func(s string, v int) (WalkFlag, int, error) {
					return WalkSet, v * 3, nil
				}
				r.Walk(fn)
			} else {
				r.Walk(testWalkHandler[int]{})
			}
		}(i)
	}

	wg.Wait()

	var ints []int
	var fn WalkFn[int] = func(s string, v int) (WalkFlag, int, error) {
		ints = append(ints, v)
		return WalkContinue, 0, nil
	}
	r.Walk(fn)

	c.Assert(ints[:5], qt.DeepEquals, []int{0, 3, 30, 300, 303})
}

func TestWalkFlag(t *testing.T) {
	c := qt.New(t)

	f := WalkSet
	c.Assert(f.ShouldSet(), qt.IsTrue)
	c.Assert(f.ShouldStop(), qt.IsFalse)
	c.Assert(f.ShouldSkip(), qt.IsFalse)

	f = WalkSet | WalkStop
	c.Assert(f.ShouldSet(), qt.IsTrue)
	c.Assert(f.ShouldStop(), qt.IsTrue)

	f = WalkSkip
	c.Assert(f.ShouldSkip(), qt.IsTrue)
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
		var fn WalkFn[bool] = func(s string, v bool) (WalkFlag, bool, error) {
			out = append(out, s)
			return WalkContinue, false, nil
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
		r.Insert(k, "")
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
	r := New[string]()

	keys := []string{
		"foobar",
		"foo/bar/baz",
		"foo/baz/bar",
		"foo/zip/zap",
		"zipzap",
	}
	for _, k := range keys {
		r.Insert(k, "")
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
		var fn WalkFn[string] = func(s string, v string) (WalkFlag, string, error) {
			out = append(out, s)
			return WalkContinue, "", nil
		}
		r.WalkPrefix(test.inp, fn)
		sort.Strings(out)
		sort.Strings(test.out)
		c.Assert(out, qt.DeepEquals, test.out)
	}
}

func TestWalkPath(t *testing.T) {
	c := qt.New(t)
	r := New[string]()

	keys := []string{
		"foo",
		"foo/bar",
		"foo/bar/baz",
		"foo/baz/bar",
		"foo/zip/zap",
		"zipzap",
	}
	for _, k := range keys {
		r.Insert(k, "")
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
		var fn WalkFn[string] = func(s string, v string) (WalkFlag, string, error) {
			out = append(out, s)
			return WalkContinue, "", nil
		}
		r.WalkPath(test.inp, fn)
		sort.Strings(out)
		sort.Strings(test.out)
		c.Assert(out, qt.DeepEquals, test.out)
	}
}

func TestWalkDelete(t *testing.T) {
	c := qt.New(t)
	r := New[string]()
	r.Insert("init0/0", "")
	r.Insert("init0/1", "")
	r.Insert("init0/2", "")
	r.Insert("init0/3", "")
	r.Insert("init1/0", "")
	r.Insert("init1/1", "")
	r.Insert("init1/2", "")
	r.Insert("init1/3", "")
	r.Insert("init2", "")

	var deleteFn WalkFn[string] = func(s string, v string) (WalkFlag, string, error) {
		r.Delete(s)
		return WalkContinue, "", nil
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

func TestWalkErrors(t *testing.T) {
	c := qt.New(t)

	r := New[string]()
	r.Insert("init0/0", "")
	r.Insert("init0/1", "")
	r.Insert("init0/2", "")

	errWalk := fmt.Errorf("walk error")
	var walkErrFn WalkFn[string] = func(s string, v string) (WalkFlag, string, error) {
		return WalkContinue, "", errWalk
	}

	err := r.Walk(walkErrFn)
	c.Assert(err, qt.Equals, errWalk)

	errWalkPrefix := fmt.Errorf("walk prefix error")
	var walkPrefixErrFn WalkFn[string] = func(s string, v string) (WalkFlag, string, error) {
		return WalkContinue, "", errWalkPrefix
	}

	err = r.WalkPrefix("init0", walkPrefixErrFn)
	c.Assert(err, qt.Equals, errWalkPrefix)

	errWalkPath := fmt.Errorf("walk path error")
	var walkPathErrFn WalkFn[string] = func(s string, v string) (WalkFlag, string, error) {
		return WalkContinue, "", errWalkPath
	}

	err = r.WalkPath("init0/1", walkPathErrFn)
	c.Assert(err, qt.Equals, errWalkPath)
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

	var fn WalkFn[*v] = func(s string, v *v) (WalkFlag, *v, error) {
		return WalkContinue, nil, nil
	}

	skipAll := testWalkHandler[*v]{}

	b.ResetTimer()

	b.Run("Walk", func(b *testing.B) {
		for b.Loop() {
			r.Walk(fn)
		}
	})

	b.Run("Walk keys", func(b *testing.B) {
		for b.Loop() {
			r.Walk(skipAll)
		}
	})

	b.Run("WalkPrefix", func(b *testing.B) {
		for b.Loop() {
			r.WalkPrefix("init50", fn)
		}
	})

	b.Run("WalkPrefix keys", func(b *testing.B) {
		for b.Loop() {
			r.WalkPrefix("init50", skipAll)
		}
	})

	b.Run("WalkPath", func(b *testing.B) {
		for b.Loop() {
			r.WalkPath("init50/50", fn)
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

type testWalkHandler[T any] struct {
	check  func(s string) (WalkFlag, error)
	handle func(s string, v T) (WalkFlag, T, error)
}

func (w testWalkHandler[T]) Check(s string) (WalkFlag, error) {
	if w.check != nil {
		return w.check(s)
	}
	return WalkSkip, nil
}

func (w testWalkHandler[T]) Handle(s string, v T) (WalkFlag, T, error) {
	if w.handle != nil {
		return w.handle(s, v)
	}
	var zero T
	return WalkContinue, zero, nil
}
