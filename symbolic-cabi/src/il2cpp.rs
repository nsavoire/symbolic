use std::ffi::CStr;
use std::os::raw::c_char;
use std::slice;

use symbolic::{common::ByteView, il2cpp::LineMapping};

use crate::core::SymbolicStr;
use crate::utils::ForeignObject;

pub struct SymbolicLineMapping;

impl ForeignObject for SymbolicLineMapping {
    type RustObject = LineMapping;
}

#[repr(C)]
pub struct SymbolicLineMappingResult {
    pub file: SymbolicStr,
    pub line: u32,
}

ffi_fn! {
    /// Creates an archive from a byte buffer without taking ownership of the pointer.
    unsafe fn symbolic_il2cpp_line_mapping_from_bytes(
        bytes: *const u8,
        len: usize,
    ) -> Result<*mut SymbolicLineMapping> {
        let byteview = ByteView::from_slice(slice::from_raw_parts(bytes, len));
        let mapping_file = LineMapping::parse(&byteview).ok_or_else(|| anyhow::Error::msg("Invalid IL2CPP line mapping file"))?;
        Ok(SymbolicLineMapping::from_rust(mapping_file))
    }
}

ffi_fn! {
    unsafe fn symbolic_il2cpp_line_mapping_free(mapping: *mut SymbolicLineMapping) {
        SymbolicLineMapping::drop(mapping)
    }
}

ffi_fn! {
    /// Looks up a source location.
    unsafe fn symbolic_il2cpp_line_mapping_lookup(
        line_mapping_ptr: *const SymbolicLineMapping,
        file: *const c_char,
        line: u32,
    ) -> Result<SymbolicLineMappingResult> {
        let line_mapping = SymbolicLineMapping::as_rust(line_mapping_ptr);

        let result = line_mapping.lookup(CStr::from_ptr(file).to_str()?, line).ok_or_else(|| anyhow::Error::msg("Could not map source location."))?;

        Ok(SymbolicLineMappingResult {
            file: SymbolicStr::from_string(result.0.to_string()),
            line: result.1,
        })
    }
}

ffi_fn! {
    unsafe fn symbolic_il2cpp_line_mapping_result_free(result: *mut SymbolicLineMappingResult) {
        if !result.is_null() {
            let result = &*result;
            drop(result)
        }
    }
}
