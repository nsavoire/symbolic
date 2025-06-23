package symbolic

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/symbolic/pkg/null"
)

var ctx = context.Background()

func loadFixture(t *testing.T, path string) []byte {
	symbolFile, err := os.Open(path)
	require.NoError(t, err)

	bytes, err := io.ReadAll(symbolFile)
	require.NoError(t, err)

	return bytes
}

func TestLookup(t *testing.T) {
	bytes := loadFixture(t, "./fixtures/libsystem_pthread.dylib")
	archive, err := ArchiveFromBytes(ctx, bytes)
	require.NoError(t, err)

	arm64Symbols, err := archive.GetObject(ctx, 0)
	require.NoError(t, err)

	// Check UUID matches
	uuid, err := arm64Symbols.GetCodeID(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "d50357243cf433beb3c14f5502a9130f", uuid)

	symcache, err := arm64Symbols.ToSymCache(ctx)
	require.NoError(t, err)

	// Make a symbol lookup
	result, err := symcache.Lookup(ctx, 39740)
	assert.NoError(t, err)
	assert.Equal(t, []SourceLocation{{
		SymAddr:   null.ValidUint64(39452),
		InstrAddr: 39740,
		Lang:      "unknown",
		Symbol:    null.ValidString("_pthread_start"),
	}}, result)
}

func TestLookupMultipleResults(t *testing.T) {
	bytes := loadFixture(t, "./fixtures/DatadogApp")
	archive, err := ArchiveFromBytes(ctx, bytes)
	require.NoError(t, err)

	arm64Symbols, err := archive.GetObject(ctx, 0)
	require.NoError(t, err)
	// Check UUID matches
	uuid, err := arm64Symbols.GetCodeID(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "b7a5dbb8381a3813bb1c07d9972b2db6", uuid)

	symcache, err := arm64Symbols.ToSymCache(ctx)
	require.NoError(t, err)

	// Make a symbol lookup
	result, err := symcache.Lookup(ctx, 410436)
	assert.NoError(t, err)
	assert.Equal(t, []SourceLocation{
		{
			SymAddr:   null.Uint64{},
			InstrAddr: 410436,
			Line:      0,
			Lang:      "swift",
			Symbol:    null.ValidString("crashTheApp"),
			FullPath:  "/Users/vagrant/git/Targets/DatadogApp/UI/Debug Assistant/DebugMenuView.swift",
		},
		{
			SymAddr:   null.Uint64{},
			InstrAddr: 410436,
			Lang:      "swift",
			Symbol:    null.String{},
			FullPath:  "/Users/vagrant/git/Targets/DatadogApp/UI/Debug Assistant/DebugMenuView.swift",
			Line:      101,
		},
		{
			SymAddr:   null.ValidUint64(410352),
			InstrAddr: 410436,
			Line:      0,
			Lang:      "swift",
			Symbol:    null.ValidString("$s10DatadogApp13DebugMenuViewV4bodyQrvg7SwiftUI05TupleE0VyAE7SectionVyAE4TextVAE14NavigationLinkVyAA014APIEnvironmentE0VyAE05EmptyE0VGAA19APIEnvironmentsListVGAQG_AIyAkGyAC04makeM05title11destinationQrSS_xtAE0E0RzlFQOy_AA017DictionaryVisitorE0VQo__A1_A1_AcwxYQrSS_xtAeZRzlFQOy_AA05ArrayvE0VQo_AcwxYQrSS_xtAeZRzlFQOy_AC0R11SessionInfoQryFQOy_Qo_Qo_AcwxYQrSS_xtAeZRzlFQOy_AC0r15PersistedTokensY0QryFQOy_Qo_Qo_AcwxYQrSS_xtAeZRzlFQOy_AC0R20FirebaseRemoteConfigQryFQOy_Qo_Qo_tGAQGAIyAkGyAC0r12UserDefaultsM0AX12userDefaultsQrSS_So14NSUserDefaultsCtFQOy_Qo__A20_A20_tGAQGAIyAKA1_AQGAIyAkE6HStackVyAGyAK_AeZPAEE09multilineK9AlignmentyQrAE0K9AlignmentOFQOyAE0K5FieldVyAKG_Qo_AA11ClearButtonVSgtGGAQGAIyAkGyAA015AnimationsSpeedE0V_AA015TouchesSettingsE0VtGAQGAIyAkGyAE6ButtonVyAKG_A48_A48_A48_tGAQGAIyAKA48_AKGtGyXEfU_A49_yXEfU5_yycACcfu5_yycfu6_TA"),
			FullPath:  "<compiler-generated>",
		},
	}, result)
}

func TestLookupFailed(t *testing.T) {
	archive, err := ArchiveFromBytes(ctx, loadFixture(t, "./fixtures/libsystem_pthread.dylib"))
	require.NoError(t, err)

	arm64Symbols, err := archive.GetObject(ctx, 0)
	require.NoError(t, err)

	symcache, err := arm64Symbols.ToSymCache(ctx)
	require.NoError(t, err)

	// Make a symbol lookup with an inexistent address
	_, err = symcache.Lookup(ctx, 0)
	assert.True(t, errors.Is(err, ErrLookupFailed))
}

func TestArchiveLoadError(t *testing.T) {
	_, err := ArchiveFromBytes(ctx, []byte("this string definitely isn't the representation of a symbol file"))
	var symbolicErr *Error
	assert.True(t, errors.As(err, &symbolicErr))
	assert.Equal(t, int64(2100), symbolicErr.code)
	assert.Equal(t, "unsupported object file format", symbolicErr.message)
}

func TestGetObjectCount(t *testing.T) {
	archive, err := ArchiveFromBytes(ctx, loadFixture(t, "./fixtures/libsystem_pthread.dylib"))
	require.NoError(t, err)

	count, err := archive.GetObjectCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}
func TestGetObjectError(t *testing.T) {
	archive, err := ArchiveFromBytes(ctx, loadFixture(t, "./fixtures/libsystem_pthread.dylib"))
	require.NoError(t, err)

	// Get object that doesn't exist
	_, err = archive.GetObject(ctx, 5)
	assert.True(t, errors.Is(err, ErrNoSuchObject))
}

func TestGetObjectWithUUID(t *testing.T) {
	archive, err := ArchiveFromBytes(ctx, loadFixture(t, "./fixtures/libsystem_pthread.dylib"))
	require.NoError(t, err)

	_, err = archive.GetObjectWithUUID(ctx, uuid.MustParse("d5035724-3cf4-33be-b3c1-4f5502a9130f"))
	assert.NoError(t, err)
}
func TestGetObjectWithUUIDError(t *testing.T) {
	archive, err := ArchiveFromBytes(ctx, loadFixture(t, "./fixtures/libsystem_pthread.dylib"))
	require.NoError(t, err)

	obj, err := archive.GetObjectWithUUID(ctx, uuid.MustParse("12341234-1234-1234-1234-123412341234"))
	assert.Nil(t, obj)
	assert.True(t, errors.Is(err, ErrNoSuchObject))
}

func TestSymcacheSerialization(t *testing.T) {
	archive, err := ArchiveFromBytes(ctx, loadFixture(t, "./fixtures/libsystem_pthread.dylib"))
	require.NoError(t, err)

	arm64Symbols, err := archive.GetObject(ctx, 0)
	require.NoError(t, err)

	symcache, err := arm64Symbols.ToSymCache(ctx)
	require.NoError(t, err)

	bytes, err := symcache.ToBytes(ctx)
	assert.NoError(t, err)

	assert.Equal(t, loadFixture(t, "./fixtures/libsystem_pthread_arm64_v8.symcache"), bytes)

}

func TestSymcacheDeserialization(t *testing.T) {
	symcache, err := SymCacheFromBytes(ctx, loadFixture(t, "./fixtures/libsystem_pthread_arm64_v8.symcache"))
	require.NoError(t, err)

	uuid, err := symcache.GetDebugID(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "d5035724-3cf4-33be-b3c1-4f5502a9130f", uuid)

	// Make a symbol lookup
	result, err := symcache.Lookup(ctx, 39740)
	assert.NoError(t, err)
	assert.Equal(t, []SourceLocation{{
		SymAddr:   null.ValidUint64(39452),
		InstrAddr: 39740,
		Lang:      "unknown",
		Symbol:    null.ValidString("_pthread_start"),
	}}, result)
}

func TestSymcacheSize(t *testing.T) {
	symcacheBytes := loadFixture(t, "./fixtures/libsystem_pthread_arm64_v8.symcache")

	symcache, err := SymCacheFromBytes(ctx, symcacheBytes)
	require.NoError(t, err)

	size, err := symcache.Size(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(len(symcacheBytes)), size)
}

func TestGetVersion(t *testing.T) {
	symcacheBytes := loadFixture(t, "./fixtures/libsystem_pthread_arm64_v8.symcache")

	symcache, err := SymCacheFromBytes(ctx, symcacheBytes)
	require.NoError(t, err)

	version, err := symcache.GetVersion(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, uint32(8), version)
}

func TestGetLatestSymcacheVersion(t *testing.T) {
	version, err := GetLatestSymcacheVersion(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, uint32(8), version)
}

func TestSymcacheDeserializationError(t *testing.T) {
	_, err := SymCacheFromBytes(ctx, []byte("wait, this isn't a symcache..\nNever has been"))

	var symbolicErr *Error
	assert.True(t, errors.As(err, &symbolicErr))
	assert.Equal(t, int64(6002), symbolicErr.code)
	assert.Equal(t, "could not read header", symbolicErr.message)
}

func TestDemangle(t *testing.T) {
	testCases := []struct {
		name string
		exp  string
	}{
		{
			name: "__workq_kernreturn",
			exp:  "__workq_kernreturn",
		},
		{
			name: "$s9SimpleApp22CheckoutViewControllerC22didTapDoSomethingAsyncyyypFyycfU_yycfU_",
			exp:  "closure #1 () in closure #1 () in CheckoutViewController.didTapDoSomethingAsync(Any)",
		},
		{
			name: "$sIeg_IeyB_TR",
			exp:  "thunk for @escaping @callee_guaranteed () -> ()",
		},
		// too long string should not be demangled
		{
			name: "$s7Datadog16DataUploadWorkerC5queue10fileReader12dataUploader16uploadConditions5delay11featureName15internalMonitorACSo012OS_dispatch_E0C_AA0G0_pAA0bI4Type_pAA0bcK0VAA5Delay_pSSAA08InternalP0VSgtcfcyycfU_$s7Datadog16DataUploadWorkerC5queue10fileReader12dataUploader16uploadConditions5delay11featureName15internalMonitorACSo012OS_dispatch_E0C_AA0G0_pAA0bI4Type_pAA0bcK0VAA5Delay_pSSAA08InternalP0VSgtcfcyycfU_$s7Datadog16DataUploadWorkerC5queue10fileReader12dataUploader16uploadConditions5delay11featureName15internalMonitorACSo012OS_dispatch_E0C_AA0G0_pAA0bI4Type_pAA0bcK0VAA5Delay_pSSAA08InternalP0VSgtcfcyycfU_",
			exp:  "$s7Datadog16DataUploadWorkerC5queue10fileReader12dataUploader16uploadConditions5delay11featureName15internalMonitorACSo012OS_dispatch_E0C_AA0G0_pAA0bI4Type_pAA0bcK0VAA5Delay_pSSAA08InternalP0VSgtcfcyycfU_$s7Datadog16DataUploadWorkerC5queue10fileReader12dataUploader16uploadConditions5delay11featureName15internalMonitorACSo012OS_dispatch_E0C_AA0G0_pAA0bI4Type_pAA0bcK0VAA5Delay_pSSAA08InternalP0VSgtcfcyycfU_$s7Datadog16DataUploadWorkerC5queue10fileReader12dataUploader16uploadConditions5delay11featureName15internalMonitorACSo012OS_dispatch_E0C_AA0G0_pAA0bI4Type_pAA0bcK0VAA5Delay_pSSAA08InternalP0VSgtcfcyycfU_",
		},
	}
	for _, tt := range testCases {
		t.Run("demangle-"+tt.name, func(t *testing.T) {
			demangled, err := Demangle(ctx, tt.name)
			assert.NoError(t, err)
			assert.Equal(t, tt.exp, demangled)
		})
	}
}

func TestLineMappingLookup(t *testing.T) {
	bytes := loadFixture(t, "./fixtures/LineNumberMappings.json")
	mapping, err := IL2CPPLineMappingFromBytes(ctx, bytes)
	require.NoError(t, err)

	// Make a symbol lookup
	path := "/Users/jeff.ward/Projects/dd-sdk-unity/samples/Datadog Sample/Build/Android/unityLibrary/src/main/Il2CppOutputProject/Source/il2cppOutput/GenericMethods__9.cpp"
	file, line, err := mapping.Map(ctx, path, uint32(15058))
	assert.NoError(t, err)
	assert.Equal(t, file, "./Library/PackageCache/com.unity.ugui@1.0.0/Runtime/EventSystem/ExecuteEvents.cs")
	assert.Equal(t, uint32(999999), line)
}
