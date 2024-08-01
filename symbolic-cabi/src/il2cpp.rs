use std::ffi::CStr;
use std::os::raw::c_char;
use std::slice;

use symbolic::{common::ByteView, il2cpp::LineMapping};

use crate::core::SymbolicStr;
use crate::utils::ForeignObject;

pub struct SymbolicIL2CPPLineMapping;

impl ForeignObject for SymbolicIL2CPPLineMapping {
    type RustObject = LineMapping;
}

#[repr(C)]
pub struct SymbolicIL2CPPLineMappingResult {
    pub file: SymbolicStr,
    pub line: u32,
}

ffi_fn! {
    /// Creates an archive from a byte buffer without taking ownership of the pointer.
    unsafe fn symbolic_il2cpp_line_mapping_from_bytes(
        bytes: *const u8,
        len: usize,
    ) -> Result<*mut SymbolicIL2CPPLineMapping> {
        let byteview = ByteView::from_slice(slice::from_raw_parts(bytes, len));
        let mapping_file = LineMapping::parse(&byteview).ok_or_else(|| anyhow::Error::msg("Invalid IL2CPP line mapping file"))?;
        Ok(SymbolicIL2CPPLineMapping::from_rust(mapping_file))
    }
}

ffi_fn! {
    unsafe fn symbolic_il2cpp_line_mapping_free(mapping: *mut SymbolicIL2CPPLineMapping) {
        SymbolicIL2CPPLineMapping::drop(mapping)
    }
}

ffi_fn! {
    /// Looks up a source location.
    unsafe fn symbolic_il2cpp_line_mapping_lookup(
        line_mapping_ptr: *const SymbolicIL2CPPLineMapping,
        file: *const c_char,
        line: u32,
    ) -> Result<SymbolicIL2CPPLineMappingResult> {
        let line_mapping = SymbolicIL2CPPLineMapping::as_rust(line_mapping_ptr);

        let result = line_mapping.lookup(CStr::from_ptr(file).to_str()?, line).ok_or_else(|| anyhow::Error::msg("Could not map source location."))?;

        Ok(SymbolicIL2CPPLineMappingResult {
            file: SymbolicStr::from_string(result.0.to_string()),
            line: result.1,
        })
    }
}

ffi_fn! {
    unsafe fn symbolic_il2cpp_line_mapping_result_free(result: *mut SymbolicIL2CPPLineMappingResult) {
        if !result.is_null() {
            let result = &*result;
            drop(result)
        }
    }
}
