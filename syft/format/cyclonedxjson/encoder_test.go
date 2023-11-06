package cyclonedxjson

import (
	"bytes"
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anchore/syft/syft/format/internal/testutil"
	"github.com/anchore/syft/syft/sbom"
)

var updateSnapshot = flag.Bool("update-cyclonedx-json", false, "update the *.golden files for cyclone-dx JSON encoders")
var updateImage = flag.Bool("update-image", false, "update the golden image used for image encoder testing")

func getEncoder(t testing.TB) sbom.FormatEncoder {
	enc, err := NewFormatEncoderWithConfig(DefaultEncoderConfig())
	require.NoError(t, err)
	return enc
}

func TestCycloneDxDirectoryEncoder(t *testing.T) {
	dir := t.TempDir()
	testutil.AssertEncoderAgainstGoldenSnapshot(t,
		testutil.EncoderSnapshotTestConfig{
			Subject:                     testutil.DirectoryInput(t, dir),
			Format:                      getEncoder(t),
			UpdateSnapshot:              *updateSnapshot,
			PersistRedactionsInSnapshot: true,
			IsJSON:                      true,
			Redactor:                    redactor(dir),
		},
	)
}

func TestCycloneDxImageEncoder(t *testing.T) {
	testImage := "image-simple-with-labels"
	testutil.AssertEncoderAgainstGoldenImageSnapshot(t,
		testutil.ImageSnapshotTestConfig{
			Image:               testImage,
			UpdateImageSnapshot: *updateImage,
		},
		testutil.EncoderSnapshotTestConfig{
			Subject:                     testutil.ImageInput(t, testImage),
			Format:                      getEncoder(t),
			UpdateSnapshot:              *updateSnapshot,
			PersistRedactionsInSnapshot: true,
			IsJSON:                      true,
			Redactor:                    redactor(),
		},
	)
}

func redactor(values ...string) testutil.Redactor {
	return testutil.NewRedactions().
		WithValuesRedacted(values...).
		WithPatternRedactors(
			map[string]string{
				// UUIDs
				`urn:uuid:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`: `urn:uuid:redacted`,

				// timestamps
				`([0-9]+)-(0[1-9]|1[012])-(0[1-9]|[12][0-9]|3[01])[Tt]([01][0-9]|2[0-3]):([0-5][0-9]):([0-5][0-9]|60)(\.[0-9]+)?(([Zz])|([+|\-]([01][0-9]|2[0-3]):[0-5][0-9]))`: `timestamp:redacted`,

				// image hashes
				`sha256:[A-Fa-f0-9]{64}`: `sha256:redacted`,

				// BOM refs
				`"bom-ref":\s*"[^"]+"`: `"bom-ref":"redacted"`,
			},
		)
}

func TestSupportedVersions(t *testing.T) {
	encs := defaultFormatEncoders()
	require.NotEmpty(t, encs)

	versions := SupportedVersions()
	require.Equal(t, len(versions), len(encs))

	subject := testutil.DirectoryInput(t, t.TempDir())
	dec := NewFormatDecoder()

	for _, enc := range encs {
		t.Run(enc.Version(), func(t *testing.T) {
			require.Contains(t, versions, enc.Version())

			var buf bytes.Buffer
			require.NoError(t, enc.Encode(&buf, subject))

			id, version := dec.Identify(bytes.NewReader(buf.Bytes()))
			require.Equal(t, enc.ID(), id)
			require.Equal(t, enc.Version(), version)

			var s *sbom.SBOM
			var err error
			s, id, version, err = dec.Decode(bytes.NewReader(buf.Bytes()))
			require.NoError(t, err)
			require.Equal(t, enc.ID(), id)
			require.Equal(t, enc.Version(), version)

			require.NotEmpty(t, s.Artifacts.Packages.PackageCount())

			assert.Equal(t, len(subject.Relationships), len(s.Relationships), "mismatched relationship count")

			if !assert.Equal(t, subject.Artifacts.Packages.PackageCount(), s.Artifacts.Packages.PackageCount(), "mismatched package count") {
				t.Logf("expected: %d", subject.Artifacts.Packages.PackageCount())
				for _, p := range subject.Artifacts.Packages.Sorted() {
					t.Logf("  - %s", p.String())
				}
				t.Logf("actual: %d", s.Artifacts.Packages.PackageCount())
				for _, p := range s.Artifacts.Packages.Sorted() {
					t.Logf("  - %s", p.String())
				}
			}
		})
	}
}

func defaultFormatEncoders() []sbom.FormatEncoder {
	var encs []sbom.FormatEncoder
	for _, version := range SupportedVersions() {
		enc, err := NewFormatEncoderWithConfig(EncoderConfig{Version: version})
		if err != nil {
			panic(err)
		}
		encs = append(encs, enc)
	}
	return encs
}
