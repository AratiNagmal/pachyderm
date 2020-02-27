package worker

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pachyderm/pachyderm/src/client"
	"github.com/pachyderm/pachyderm/src/client/pkg/require"
	"github.com/pachyderm/pachyderm/src/client/pps"
	tu "github.com/pachyderm/pachyderm/src/server/pkg/testutil"
)

func TestDatumIterators(t *testing.T) {
	c := tu.GetPachClient(t)
	defer require.NoError(t, c.DeleteAll())

	dataRepo := tu.UniqueString(t.Name() + "_data")
	require.NoError(t, c.CreateRepo(dataRepo))

	// put files in structured in a way so that there are many ways to glob it
	commit, err := c.StartCommit(dataRepo, "master")
	require.NoError(t, err)
	for j := 0; j < 50; j++ {
		_, err = c.PutFile(dataRepo, commit.ID, fmt.Sprintf("foo%v", j), strings.NewReader("bar"))
		require.NoError(t, err)
	}
	require.NoError(t, err)
	require.NoError(t, c.FinishCommit(dataRepo, commit.ID))

	// in0 has zero datums, for testing edge cases
	in0 := client.NewPFSInput(dataRepo, "!(**)")
	in0.Pfs.Commit = commit.ID
	t.Run("ZeroDatums", func(t *testing.T) {
		pfs0, err := NewDatumIterator(c, in0)
		require.NoError(t, err)

		validateDI(t, pfs0)
	})

	// in[1-2] are basic PFS inputs
	in1 := client.NewPFSInput(dataRepo, "/foo?1")
	in1.Pfs.Commit = commit.ID
	in2 := client.NewPFSInput(dataRepo, "/foo*2")
	in2.Pfs.Commit = commit.ID
	t.Run("Basic", func(t *testing.T) {
		pfs1, err := NewDatumIterator(c, in1)
		require.NoError(t, err)
		pfs2, err := NewDatumIterator(c, in2)
		require.NoError(t, err)

		// iterate through pfs0, pfs1 and pfs2 and verify they are as we expect
		validateDI(t, pfs1, "/foo11", "/foo21", "/foo31", "/foo41")
		validateDI(t, pfs2, "/foo12", "/foo2", "/foo22", "/foo32", "/foo42")
	})

	in3 := client.NewUnionInput(in1, in2)
	t.Run("Union", func(t *testing.T) {
		union1, err := NewDatumIterator(c, in3)
		require.NoError(t, err)
		validateDI(t, union1, "/foo11", "/foo21", "/foo31", "/foo41",
			"/foo12", "/foo2", "/foo22", "/foo32", "/foo42")
	})

	in4 := client.NewCrossInput(in1, in2)
	t.Run("Cross", func(t *testing.T) {
		cross1, err := NewDatumIterator(c, in4)
		require.NoError(t, err)
		validateDI(t, cross1,
			"/foo11/foo12", "/foo21/foo12", "/foo31/foo12", "/foo41/foo12",
			"/foo11/foo2", "/foo21/foo2", "/foo31/foo2", "/foo41/foo2",
			"/foo11/foo22", "/foo21/foo22", "/foo31/foo22", "/foo41/foo22",
			"/foo11/foo32", "/foo21/foo32", "/foo31/foo32", "/foo41/foo32",
			"/foo11/foo42", "/foo21/foo42", "/foo31/foo42", "/foo41/foo42",
		)
	})

	// in5 is a nested cross
	in5 := client.NewCrossInput(in3, in4)
	t.Run("NestedCross", func(t *testing.T) {
		cross2, err := NewDatumIterator(c, in5)
		require.NoError(t, err)
		validateDI(t, cross2,
			"/foo11/foo11/foo12", "/foo21/foo11/foo12", "/foo31/foo11/foo12", "/foo41/foo11/foo12", "/foo12/foo11/foo12", "/foo2/foo11/foo12", "/foo22/foo11/foo12", "/foo32/foo11/foo12", "/foo42/foo11/foo12",
			"/foo11/foo21/foo12", "/foo21/foo21/foo12", "/foo31/foo21/foo12", "/foo41/foo21/foo12", "/foo12/foo21/foo12", "/foo2/foo21/foo12", "/foo22/foo21/foo12", "/foo32/foo21/foo12", "/foo42/foo21/foo12",
			"/foo11/foo31/foo12", "/foo21/foo31/foo12", "/foo31/foo31/foo12", "/foo41/foo31/foo12", "/foo12/foo31/foo12", "/foo2/foo31/foo12", "/foo22/foo31/foo12", "/foo32/foo31/foo12", "/foo42/foo31/foo12",
			"/foo11/foo41/foo12", "/foo21/foo41/foo12", "/foo31/foo41/foo12", "/foo41/foo41/foo12", "/foo12/foo41/foo12", "/foo2/foo41/foo12", "/foo22/foo41/foo12", "/foo32/foo41/foo12", "/foo42/foo41/foo12",
			"/foo11/foo11/foo2", "/foo21/foo11/foo2", "/foo31/foo11/foo2", "/foo41/foo11/foo2", "/foo12/foo11/foo2", "/foo2/foo11/foo2", "/foo22/foo11/foo2", "/foo32/foo11/foo2", "/foo42/foo11/foo2",
			"/foo11/foo21/foo2", "/foo21/foo21/foo2", "/foo31/foo21/foo2", "/foo41/foo21/foo2", "/foo12/foo21/foo2", "/foo2/foo21/foo2", "/foo22/foo21/foo2", "/foo32/foo21/foo2", "/foo42/foo21/foo2",
			"/foo11/foo31/foo2", "/foo21/foo31/foo2", "/foo31/foo31/foo2", "/foo41/foo31/foo2", "/foo12/foo31/foo2", "/foo2/foo31/foo2", "/foo22/foo31/foo2", "/foo32/foo31/foo2", "/foo42/foo31/foo2",
			"/foo11/foo41/foo2", "/foo21/foo41/foo2", "/foo31/foo41/foo2", "/foo41/foo41/foo2", "/foo12/foo41/foo2", "/foo2/foo41/foo2", "/foo22/foo41/foo2", "/foo32/foo41/foo2", "/foo42/foo41/foo2",
			"/foo11/foo11/foo22", "/foo21/foo11/foo22", "/foo31/foo11/foo22", "/foo41/foo11/foo22", "/foo12/foo11/foo22", "/foo2/foo11/foo22", "/foo22/foo11/foo22", "/foo32/foo11/foo22", "/foo42/foo11/foo22",
			"/foo11/foo21/foo22", "/foo21/foo21/foo22", "/foo31/foo21/foo22", "/foo41/foo21/foo22", "/foo12/foo21/foo22", "/foo2/foo21/foo22", "/foo22/foo21/foo22", "/foo32/foo21/foo22", "/foo42/foo21/foo22",
			"/foo11/foo31/foo22", "/foo21/foo31/foo22", "/foo31/foo31/foo22", "/foo41/foo31/foo22", "/foo12/foo31/foo22", "/foo2/foo31/foo22", "/foo22/foo31/foo22", "/foo32/foo31/foo22", "/foo42/foo31/foo22",
			"/foo11/foo41/foo22", "/foo21/foo41/foo22", "/foo31/foo41/foo22", "/foo41/foo41/foo22", "/foo12/foo41/foo22", "/foo2/foo41/foo22", "/foo22/foo41/foo22", "/foo32/foo41/foo22", "/foo42/foo41/foo22",
			"/foo11/foo11/foo32", "/foo21/foo11/foo32", "/foo31/foo11/foo32", "/foo41/foo11/foo32", "/foo12/foo11/foo32", "/foo2/foo11/foo32", "/foo22/foo11/foo32", "/foo32/foo11/foo32", "/foo42/foo11/foo32",
			"/foo11/foo21/foo32", "/foo21/foo21/foo32", "/foo31/foo21/foo32", "/foo41/foo21/foo32", "/foo12/foo21/foo32", "/foo2/foo21/foo32", "/foo22/foo21/foo32", "/foo32/foo21/foo32", "/foo42/foo21/foo32",
			"/foo11/foo31/foo32", "/foo21/foo31/foo32", "/foo31/foo31/foo32", "/foo41/foo31/foo32", "/foo12/foo31/foo32", "/foo2/foo31/foo32", "/foo22/foo31/foo32", "/foo32/foo31/foo32", "/foo42/foo31/foo32",
			"/foo11/foo41/foo32", "/foo21/foo41/foo32", "/foo31/foo41/foo32", "/foo41/foo41/foo32", "/foo12/foo41/foo32", "/foo2/foo41/foo32", "/foo22/foo41/foo32", "/foo32/foo41/foo32", "/foo42/foo41/foo32",
			"/foo11/foo11/foo42", "/foo21/foo11/foo42", "/foo31/foo11/foo42", "/foo41/foo11/foo42", "/foo12/foo11/foo42", "/foo2/foo11/foo42", "/foo22/foo11/foo42", "/foo32/foo11/foo42", "/foo42/foo11/foo42",
			"/foo11/foo21/foo42", "/foo21/foo21/foo42", "/foo31/foo21/foo42", "/foo41/foo21/foo42", "/foo12/foo21/foo42", "/foo2/foo21/foo42", "/foo22/foo21/foo42", "/foo32/foo21/foo42", "/foo42/foo21/foo42",
			"/foo11/foo31/foo42", "/foo21/foo31/foo42", "/foo31/foo31/foo42", "/foo41/foo31/foo42", "/foo12/foo31/foo42", "/foo2/foo31/foo42", "/foo22/foo31/foo42", "/foo32/foo31/foo42", "/foo42/foo31/foo42",
			"/foo11/foo41/foo42", "/foo21/foo41/foo42", "/foo31/foo41/foo42", "/foo41/foo41/foo42", "/foo12/foo41/foo42", "/foo2/foo41/foo42", "/foo22/foo41/foo42", "/foo32/foo41/foo42", "/foo42/foo41/foo42")
	})

	// in6 is a cross with a zero datum input (should also be zero)
	in6 := client.NewCrossInput(in3, in0, in2, in4)
	t.Run("EmptyCross", func(t *testing.T) {
		cross3, err := NewDatumIterator(c, in6)
		require.NoError(t, err)
		validateDI(t, cross3)
	})

	// in7 is a cross with a [nested cross w/ a zero datum input]
	// (should also be zero)
	in7 := client.NewCrossInput(in6, in1)
	t.Run("NestedEmptyCross", func(t *testing.T) {
		cross4, err := NewDatumIterator(c, in7)
		require.NoError(t, err)
		validateDI(t, cross4)
	})

	// in[8-9] are elements of in10, which is a join input
	in8 := client.NewPFSInputOpts("", dataRepo, "", "/foo(?)(?)", "$1$2", false)
	in8.Pfs.Commit = commit.ID
	in9 := client.NewPFSInputOpts("", dataRepo, "", "/foo(?)(?)", "$2$1", false)
	in9.Pfs.Commit = commit.ID
	in10 := client.NewJoinInput(in8, in9)
	t.Run("Join", func(t *testing.T) {
		join1, err := NewDatumIterator(c, in10)
		require.NoError(t, err)
		validateDI(t, join1,
			"/foo11/foo11",
			"/foo12/foo21",
			"/foo13/foo31",
			"/foo14/foo41",
			"/foo21/foo12",
			"/foo22/foo22",
			"/foo23/foo32",
			"/foo24/foo42",
			"/foo31/foo13",
			"/foo32/foo23",
			"/foo33/foo33",
			"/foo34/foo43",
			"/foo41/foo14",
			"/foo42/foo24",
			"/foo43/foo34",
			"/foo44/foo44")
	})

	// in11 is an S3 input
	in11 := client.NewS3PFSInput("", dataRepo, "")
	t.Run("PlainS3", func(t *testing.T) {
		s3itr, err := NewDatumIterator(c, in11)
		require.NoError(t, err)
		validateDI(t, s3itr, "")
	})

	// in12 and in13 are a cross and union that contain an S3 input and two non-s3
	// inputs
	in12 := client.NewCrossInput(in1, in2, in11)
	in13 := client.NewUnionInput(in1, in2, in11)
	in14 := client.NewJoinInput(in8, in9, in11)
	t.Run("S3MixedCrossUnionJoin", func(t *testing.T) {
		s3CrossItr, err := NewDatumIterator(c, in12)
		require.NoError(t, err)
		s3UnionItr, err := NewDatumIterator(c, in13)
		require.NoError(t, err)
		s3JoinItr, err := NewDatumIterator(c, in14)
		require.NoError(t, err)
		require.NoError(t, err)
		validateDI(t, s3CrossItr,
			"/foo11/foo12", "/foo21/foo12", "/foo31/foo12", "/foo41/foo12",
			"/foo11/foo2", "/foo21/foo2", "/foo31/foo2", "/foo41/foo2",
			"/foo11/foo22", "/foo21/foo22", "/foo31/foo22", "/foo41/foo22",
			"/foo11/foo32", "/foo21/foo32", "/foo31/foo32", "/foo41/foo32",
			"/foo11/foo42", "/foo21/foo42", "/foo31/foo42", "/foo41/foo42",
		)
		validateDI(t, s3UnionItr, "/foo11", "/foo21", "/foo31", "/foo41",
			"/foo12", "/foo2", "/foo22", "/foo32", "/foo42")
		validateDI(t, s3JoinItr,
			"/foo11/foo11", "/foo12/foo21", "/foo13/foo31", "/foo14/foo41",
			"/foo21/foo12", "/foo22/foo22", "/foo23/foo32", "/foo24/foo42",
			"/foo31/foo13", "/foo32/foo23", "/foo33/foo33", "/foo34/foo43",
			"/foo41/foo14", "/foo42/foo24", "/foo43/foo34", "/foo44/foo44")
	})

	// in15 is a cross consisting of exclusively S3 inputs, and in16 is a union
	// consisting of exclusively s3 inputs.
	dataRepo2 := tu.UniqueString(t.Name() + "_2_data")
	require.NoError(t, c.CreateRepo(dataRepo2))
	in15 := client.NewCrossInput(in11, client.NewS3PFSInput("", dataRepo2, ""))
	in16 := client.NewUnionInput(in11, client.NewS3PFSInput("", dataRepo2, ""))
	in17 := client.NewJoinInput(in11, client.NewS3PFSInput("", dataRepo2, ""))
	t.Run("S3OnlyCrossUnionJoin", func(t *testing.T) {
		s3CrossItr, err := NewDatumIterator(c, in15)
		require.NoError(t, err)
		s3UnionItr, err := NewDatumIterator(c, in16)
		require.NoError(t, err)
		s3JoinItr, err := NewDatumIterator(c, in17)
		require.NoError(t, err)
		require.NoError(t, err)
		validateDI(t, s3CrossItr, "")
		validateDI(t, s3UnionItr, "")
		validateDI(t, s3JoinItr, "")
	})

	// in18 and in19 are crosses containing another cross. In in18, the inner
	// cross is mixed (some S3 inputs, some not) and in in19, the inner cross
	// consists of exclusively s3 inputs.
	in18 := client.NewCrossInput(in1, client.NewCrossInput(
		in11, in1))
	in19 := client.NewCrossInput(in1)
	t.Run("NestedS3", func(t *testing.T) {
		s3InnerMixedCrossItr, err := NewDatumIterator(c, in18)
		require.NoError(t, err)
		s3InnerS3OnlyCrossItr, err := NewDatumIterator(c, in19)
		require.NoError(t, err)
		validateDI(t, s3InnerMixedCrossItr,
			"/foo11/foo11", "/foo21/foo11", "/foo31/foo11", "/foo41/foo11",
			"/foo11/foo21", "/foo21/foo21", "/foo31/foo21", "/foo41/foo21",
			"/foo11/foo31", "/foo21/foo31", "/foo31/foo31", "/foo41/foo31",
			"/foo11/foo41", "/foo21/foo41", "/foo31/foo41", "/foo41/foo41",
		)
		validateDI(t, s3InnerS3OnlyCrossItr, "/foo11", "/foo21", "/foo31", "/foo41")
	})
}

func benchmarkDatumIterators(j int, b *testing.B) {
	c := tu.GetPachClient(b)
	defer require.NoError(b, c.DeleteAll())
	require.NoError(b, activateEnterprise(c))

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		dataRepo := tu.UniqueString("TestDatumIteratorPFS_data")
		require.NoError(b, c.CreateRepo(dataRepo))

		// put files in structured in a way so that there are many ways to glob it
		commit, err := c.StartCommit(dataRepo, "master")
		require.NoError(b, err)
		for i := 0; i < 100*j; i++ {
			_, err = c.PutFile(dataRepo, commit.ID, fmt.Sprintf("foo%v", i), strings.NewReader("bar"))
			require.NoError(b, err)
		}

		require.NoError(b, err)
		require.NoError(b, c.FinishCommit(dataRepo, commit.ID))

		// make one with zero datums for testing edge cases
		in0 := client.NewPFSInput(dataRepo, "!(**)")
		in0.Pfs.Commit = commit.ID
		pfs0, err := NewDatumIterator(c, in0)
		require.NoError(b, err)

		in1 := client.NewPFSInput(dataRepo, "/foo?1*")
		in1.Pfs.Commit = commit.ID
		pfs1, err := NewDatumIterator(c, in1)
		require.NoError(b, err)

		in2 := client.NewPFSInput(dataRepo, "/foo*2")
		in2.Pfs.Commit = commit.ID
		pfs2, err := NewDatumIterator(c, in2)
		require.NoError(b, err)

		validateDI(b, pfs0)
		validateDI(b, pfs1)
		validateDI(b, pfs2)

		b.Run("union", func(b *testing.B) {
			in3 := client.NewUnionInput(in1, in2)
			union1, err := NewDatumIterator(c, in3)
			require.NoError(b, err)
			validateDI(b, union1)
		})

		b.Run("cross", func(b *testing.B) {
			in4 := client.NewCrossInput(in1, in2)
			cross1, err := NewDatumIterator(c, in4)
			require.NoError(b, err)
			validateDI(b, cross1)
		})

		b.Run("join", func(b *testing.B) {
			in8 := client.NewPFSInputOpts("", dataRepo, "", "/foo(?)(?)*", "$1$2", false)
			in8.Pfs.Commit = commit.ID
			in9 := client.NewPFSInputOpts("", dataRepo, "", "/foo(?)(?)*", "$2$1", false)
			in9.Pfs.Commit = commit.ID
			join1, err := newJoinDatumIterator(c, []*pps.Input{in8, in9})
			require.NoError(b, err)
			validateDI(b, join1)
		})

		b.Run("iterated", func(b *testing.B) {
			in3 := client.NewUnionInput(in1, in2)
			in4 := client.NewCrossInput(in1, in2)

			in5 := client.NewCrossInput(in3, in4)
			cross2, err := NewDatumIterator(c, in5)
			require.NoError(b, err)

			// cross with a zero datum input should also be zero
			in6 := client.NewCrossInput(in3, in0, in2, in4)
			cross3, err := NewDatumIterator(c, in6)
			require.NoError(b, err)

			// zero cross inside a cross should also be zero
			in7 := client.NewCrossInput(in6, in1)
			cross4, err := NewDatumIterator(c, in7)
			require.NoError(b, err)

			validateDI(b, cross2)
			validateDI(b, cross3)
			validateDI(b, cross4)

		})
	}
}

func BenchmarkDI1(b *testing.B)  { benchmarkDatumIterators(1, b) }
func BenchmarkDI2(b *testing.B)  { benchmarkDatumIterators(2, b) }
func BenchmarkDI4(b *testing.B)  { benchmarkDatumIterators(4, b) }
func BenchmarkDI8(b *testing.B)  { benchmarkDatumIterators(8, b) }
func BenchmarkDI16(b *testing.B) { benchmarkDatumIterators(16, b) }
func BenchmarkDI32(b *testing.B) { benchmarkDatumIterators(32, b) }

func validateDI(t testing.TB, di DatumIterator, datums ...string) {
	t.Helper()
	i := 0
	clone := di
	for di.Next() {
		key := ""
		for _, file := range di.Datum() {
			key += file.FileInfo.File.Path
		}

		key2 := ""
		clone.DatumN(0)
		for _, file := range clone.DatumN(i) {
			key2 += file.FileInfo.File.Path
		}

		if len(datums) > 0 {
			require.Equal(t, key, datums[i])
		}
		require.Equal(t, key, key2)
		i++
	}
	if len(datums) > 0 {
		require.Equal(t, len(datums), di.Len())
	}
	require.Equal(t, i, di.Len())
}
