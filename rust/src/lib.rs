use rayon::prelude::*;
use serde::{Deseridalize, Serialize};
use std::collections::HashMap;
use std::ffi::{CStr, CString};
use std::os::raw::c_char;

pub mod compression;
pub mod crypto;
pub mod diff;

use compression::Compressor;
use crypto::CryptoCompressor;
use diff::DiffCalculator;

/// 文件元数据结构
#[derive(Debug,Clone,Serialize,Deseridalize)]
pub struct FileMetadata{
    pub path: String,
    pub hash: String,
    pub size: i64,
    pub modified_time: String,
    pub permissions: String,
}

/// 文件差异结构
#[derive(Debug,Clone,Serialize,Deseridalize)]
pub struct FileDiff{
    pub path: String,
    pub operation: String,
    pub source_hash: String,
    pub dest_hash:String,
    pub size: i64,
}

/// 操作结果结构
#[derive(Debug, Serialize, Deserialize)]
pub struct OperationResult {
    pub success: bool,
    pub message: String,
    pub data: Option<String>,
}

/// 差异计算结果
#[derive(Debug, Serialize, Deserialize)]
pub struct DiffResult {
    pub differences: Vec<FileDiff>,
    pub statistics: DiffStatistics,
}

/// 差异统计信息
#[derive(Debug, Serialize, Deserialize)]
pub struct DiffStatistics {
    pub total_source_files: usize,
    pub total_dest_files: usize,
    pub files_to_create: usize,
    pub files_to_update: usize,
    pub files_to_delete: usize,
    pub total_size: i64,
}

/// 主要的性能模块结构
pub struct SyncEngine {
    diff_calculator: DiffCalculator,
    crypto_compressor: CryptoCompressor,
    compressor: Compressor,
}

impl SyncEngine {
    /// 创建新的同步引擎实例
    pub fn new() -> Self {
        Self {
            diff_calculator: DiffCalculator::new(),
            crypto_compressor: CryptoCompressor::new(),
            compressor: Compressor::new(),
        }
    }

    /// 计算文件差异
    pub fn calculate_differences(
        &self,
        source_files: &[FileMetadata],
        dest_files: &[FileMetadata],
    ) -> Result<DiffResult, String> {
        self.diff_calculator.calculate_differences(source_files, dest_files)
    }

    /// 加密文件
    pub fn encrypt_file(&self, file_path: &str, key: &[u8]) -> Result<Vec<u8>, String> {
        self.crypto_compressor.encrypt_file(file_path, key)
    }

    /// 解密文件
    pub fn decrypt_file(&self, encrypted_data: &[u8], key: &[u8]) -> Result<Vec<u8>, String> {
        self.crypto_compressor.decrypt_data(encrypted_data, key)
    }

    /// 压缩数据
    pub fn compress_data(&self, data: &[u8]) -> Result<Vec<u8>, String> {
        self.compressor.compress(data)
    }

    /// 解压缩数据
    pub fn decompress_data(&self, compressed_data: &[u8]) -> Result<Vec<u8>, String> {
        self.compressor.decompress(compressed_data)
    }
}

/// 辅助函数：将 Rust 字符串转换为 C 字符串
fn to_c_string(s: String) -> *mut c_char {
    match CString::new(s) {
        Ok(c_string) => c_string.into_raw(),
        Err(_) => std::ptr::null_mut(),
    }
}

/// 辅助函数：从 C 字符串获取 Rust 字符串
fn from_c_string(c_str: *const c_char) -> Result<String, String> {
    if c_str.is_null() {
        return Err("空指针".to_string());
    }
    
    unsafe {
        match CStr::from_ptr(c_str).to_str() {
            Ok(s) => Ok(s.to_string()),
            Err(_) => Err("无效的UTF-8字符串".to_string()),
        }
    }
}

// ============================================================================
// C FFI 接口 - 供 Go 调用
// ============================================================================

/// C FFI: 计算文件差异
#[no_mangle]
pub extern "C" fn calculate_diff(
    source_files_json: *const c_char,
    dest_files_json: *const c_char,
) -> *mut c_char {
    let source_json = match from_c_string(source_files_json) {
        Ok(s) => s,
        Err(e) => {
            let result = OperationResult {
                success: false,
                message: format!("解析源文件列表失败: {}", e),
                data: None,
            };
            return to_c_string(serde_json::to_string(&result).unwrap_or_default());
        }
    };

    let dest_json = match from_c_string(dest_files_json) {
        Ok(s) => s,
        Err(e) => {
            let result = OperationResult {
                success: false,
                message: format!("解析目标文件列表失败: {}", e),
                data: None,
            };
            return to_c_string(serde_json::to_string(&result).unwrap_or_default());
        }
    };

    // 解析 JSON
    let source_files: Vec<FileMetadata> = match serde_json::from_str(&source_json) {
        Ok(files) => files,
        Err(e) => {
            let result = OperationResult {
                success: false,
                message: format!("反序列化源文件失败: {}", e),
                data: None,
            };
            return to_c_string(serde_json::to_string(&result).unwrap_or_default());
        }
    };

    let dest_files: Vec<FileMetadata> = match serde_json::from_str(&dest_json) {
        Ok(files) => files,
        Err(e) => {
            let result = OperationResult {
                success: false,
                message: format!("反序列化目标文件失败: {}", e),
                data: None,
            };
            return to_c_string(serde_json::to_string(&result).unwrap_or_default());
        }
    };

    // 计算差异
    let engine = SyncEngine::new();
    match engine.calculate_differences(&source_files, &dest_files) {
        Ok(diff_result) => {
            let result = OperationResult {
                success: true,
                message: "差异计算完成".to_string(),
                data: Some(serde_json::to_string(&diff_result).unwrap_or_default()),
            };
            to_c_string(serde_json::to_string(&result).unwrap_or_default())
        }
        Err(e) => {
            let result = OperationResult {
                success: false,
                message: format!("差异计算失败: {}", e),
                data: None,
            };
            to_c_string(serde_json::to_string(&result).unwrap_or_default())
        }
    }
}

/// C FFI: 加密文件
#[no_mangle]
pub extern "C" fn encrypt_file(
    file_path: *const c_char,
    key: *const c_char,
) -> *mut c_char {
    let path = match from_c_string(file_path) {
        Ok(s) => s,
        Err(e) => {
            let result = OperationResult {
                success: false,
                message: format!("解析文件路径失败: {}", e),
                data: None,
            };
            return to_c_string(serde_json::to_string(&result).unwrap_or_default());
        }
    };

    let key_str = match from_c_string(key) {
        Ok(s) => s,
        Err(e) => {
            let result = OperationResult {
                success: false,
                message: format!("解析密钥失败: {}", e),
                data: None,
            };
            return to_c_string(serde_json::to_string(&result).unwrap_or_default());
        }
    };

    let engine = SyncEngine::new();
    match engine.encrypt_file(&path, key_str.as_bytes()) {
        Ok(encrypted_data) => {
            let encoded = base64::encode(&encrypted_data);
            let result = OperationResult {
                success: true,
                message: "文件加密完成".to_string(),
                data: Some(encoded),
            };
            to_c_string(serde_json::to_string(&result).unwrap_or_default())
        }
        Err(e) => {
            let result = OperationResult {
                success: false,
                message: format!("文件加密失败: {}", e),
                data: None,
            };
            to_c_string(serde_json::to_string(&result).unwrap_or_default())
        }
    }
}

/// C FFI: 压缩文件
#[no_mangle]
pub extern "C" fn compress_file(file_path: *const c_char) -> *mut c_char {
    let path = match from_c_string(file_path) {
        Ok(s) => s,
        Err(e) => {
            let result = OperationResult {
                success: false,
                message: format!("解析文件路径失败: {}", e),
                data: None,
            };
            return to_c_string(serde_json::to_string(&result).unwrap_or_default());
        }
    };

    // 读取文件
    let file_data = match std::fs::read(&path) {
        Ok(data) => data,
        Err(e) => {
            let result = OperationResult {
                success: false,
                message: format!("读取文件失败: {}", e),
                data: None,
            };
            return to_c_string(serde_json::to_string(&result).unwrap_or_default());
        }
    };

    let engine = SyncEngine::new();
    match engine.compress_data(&file_data) {
        Ok(compressed_data) => {
            let encoded = base64::encode(&compressed_data);
            let result = OperationResult {
                success: true,
                message: "文件压缩完成".to_string(),
                data: Some(encoded),
            };
            to_c_string(serde_json::to_string(&result).unwrap_or_default())
        }
        Err(e) => {
            let result = OperationResult {
                success: false,
                message: format!("文件压缩失败: {}", e),
                data: None,
            };
            to_c_string(serde_json::to_string(&result).unwrap_or_default())
        }
    }
}

/// C FFI: 释放字符串内存
#[no_mangle]
pub extern "C" fn free_string(s: *mut c_char) {
    if !s.is_null() {
        unsafe {
            let _ = CString::from_raw(s);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_sync_engine_creation() {
        let engine = SyncEngine::new();
        // 基本的创建测试
        assert!(true);
    }

    #[test]
    fn test_file_metadata_serialization() {
        let metadata = FileMetadata {
            path: "/test/file.txt".to_string(),
            hash: "abc123".to_string(),
            size: 1024,
            modified_time: "2023-01-01T00:00:00Z".to_string(),
            permissions: "0644".to_string(),
        };

        let json = serde_json::to_string(&metadata).unwrap();
        let deserialized: FileMetadata = serde_json::from_str(&json).unwrap();
        
        assert_eq!(metadata.path, deserialized.path);
        assert_eq!(metadata.hash, deserialized.hash);
    }
}

