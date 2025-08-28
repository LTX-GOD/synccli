use flate2::Compression;
use flate2::read::{ZlibDecoder, ZlibEncoder};
use flate2::write::{ZlibDecoder as ZlibDecoderWrite, ZlibEncoder as ZlibEncoderWrite};
use std::fs;
use std::io::{Read, Write};

/// 压缩器
pub struct Compressor {
    compression_level: Compression,
}

impl Compressor {
    /// 创建新的压缩器
    pub fn new() -> Self {
        Self {
            compression_level: Compression::default(),
        }
    }

    /// 创建带有指定压缩级别的压缩器
    pub fn with_level(level: u32) -> Self {
        Self {
            compression_level: Compression::new(level),
        }
    }

    /// 压缩数据
    pub fn compress(&self, data: &[u8]) -> Result<Vec<u8>, String> {
        let mut encoder = ZlibEncoder::new(data, self.compression_level);
        let mut compressed_data = Vec::new();

        encoder
            .read_to_end(&mut compressed_data)
            .map_err(|e| format!("压缩失败: {}", e))?;

        Ok(compressed_data)
    }

    /// 解压缩数据
    pub fn decompress(&self, compressed_data: &[u8]) -> Result<Vec<u8>, String> {
        let mut decoder = ZlibDecoder::new(compressed_data);
        let mut decompressed_data = Vec::new();

        decoder
            .read_to_end(&mut decompressed_data)
            .map_err(|e| format!("解压缩失败: {}", e))?;

        Ok(decompressed_data)
    }

    /// 压缩文件
    pub fn compress_file(&self, file_path: &str) -> Result<Vec<u8>, String> {
        let file_data =
            fs::read(file_path).map_err(|e| format!("读取文件失败 {}: {}", file_path, e))?;

        self.compress(&file_data)
    }

    /// 解压缩到文件
    pub fn decompress_to_file(
        &self,
        compressed_data: &[u8],
        output_path: &str,
    ) -> Result<(), String> {
        let decompressed_data = self.decompress(compressed_data)?;

        fs::write(output_path, decompressed_data)
            .map_err(|e| format!("写入文件失败 {}: {}", output_path, e))?;

        Ok(())
    }

    /// 流式压缩（用于大文件）
    pub fn compress_stream(&self, input_data: &[u8]) -> Result<Vec<u8>, String> {
        let mut output = Vec::new();
        {
            let mut encoder = ZlibEncoderWrite::new(&mut output, self.compression_level);
            encoder
                .write_all(input_data)
                .map_err(|e| format!("流式压缩写入失败: {}", e))?;
            encoder
                .finish()
                .map_err(|e| format!("流式压缩完成失败: {}", e))?;
        }
        Ok(output)
    }

    /// 流式解压缩
    pub fn decompress_stream(&self, compressed_data: &[u8]) -> Result<Vec<u8>, String> {
        let mut output = Vec::new();
        {
            let mut decoder = ZlibDecoderWrite::new(&mut output);
            decoder
                .write_all(compressed_data)
                .map_err(|e| format!("流式解压缩写入失败: {}", e))?;
            decoder
                .finish()
                .map_err(|e| format!("流式解压缩完成失败: {}", e))?;
        }
        Ok(output)
    }

    /// 计算压缩比
    pub fn calculate_compression_ratio(&self, original_size: usize, compressed_size: usize) -> f64 {
        if original_size == 0 {
            return 0.0;
        }
        (original_size as f64 - compressed_size as f64) / original_size as f64 * 100.0
    }

    /// 压缩并返回统计信息
    pub fn compress_with_stats(&self, data: &[u8]) -> Result<CompressionResult, String> {
        let original_size = data.len();
        let compressed_data = self.compress(data)?;
        let compressed_size = compressed_data.len();
        let compression_ratio = self.calculate_compression_ratio(original_size, compressed_size);

        Ok(CompressionResult {
            compressed_data,
            original_size,
            compressed_size,
            compression_ratio,
            compression_level: self.compression_level.level(),
        })
    }

    /// 批量压缩文件
    pub fn compress_multiple_files(
        &self,
        file_paths: &[String],
    ) -> Result<Vec<FileCompressionResult>, String> {
        let mut results = Vec::new();

        for file_path in file_paths {
            match self.compress_file(file_path) {
                Ok(compressed_data) => {
                    let original_size = fs::metadata(file_path)
                        .map_err(|e| format!("获取文件元数据失败 {}: {}", file_path, e))?
                        .len() as usize;

                    let compressed_size = compressed_data.len();
                    let compression_ratio =
                        self.calculate_compression_ratio(original_size, compressed_size);

                    results.push(FileCompressionResult {
                        file_path: file_path.clone(),
                        success: true,
                        compressed_data: Some(compressed_data),
                        original_size,
                        compressed_size,
                        compression_ratio,
                        error_message: None,
                    });
                }
                Err(e) => {
                    results.push(FileCompressionResult {
                        file_path: file_path.clone(),
                        success: false,
                        compressed_data: None,
                        original_size: 0,
                        compressed_size: 0,
                        compression_ratio: 0.0,
                        error_message: Some(e),
                    });
                }
            }
        }

        Ok(results)
    }

    /// 检查数据是否已压缩
    pub fn is_compressed(&self, data: &[u8]) -> bool {
        // 简单的启发式检查：尝试解压缩前几个字节
        if data.len() < 10 {
            return false;
        }

        // zlib 数据通常以特定的字节开始
        match data[0] {
            0x78 => true, // zlib 压缩数据的常见开始字节
            _ => false,
        }
    }

    /// 自适应压缩（根据数据类型选择最佳压缩级别）
    pub fn adaptive_compress(&self, data: &[u8]) -> Result<Vec<u8>, String> {
        // 根据数据大小和类型选择压缩级别
        let compression_level = if data.len() < 1024 {
            // 小文件使用快速压缩
            Compression::fast()
        } else if data.len() > 10 * 1024 * 1024 {
            // 大文件使用最佳压缩
            Compression::best()
        } else {
            // 中等文件使用默认压缩
            Compression::default()
        };

        let mut encoder = ZlibEncoder::new(data, compression_level);
        let mut compressed_data = Vec::new();

        encoder
            .read_to_end(&mut compressed_data)
            .map_err(|e| format!("自适应压缩失败: {}", e))?;

        Ok(compressed_data)
    }
}

/// 压缩结果
#[derive(Debug, Clone)]
pub struct CompressionResult {
    pub compressed_data: Vec<u8>,
    pub original_size: usize,
    pub compressed_size: usize,
    pub compression_ratio: f64,
    pub compression_level: u32,
}

/// 文件压缩结果
#[derive(Debug, Clone)]
pub struct FileCompressionResult {
    pub file_path: String,
    pub success: bool,
    pub compressed_data: Option<Vec<u8>>,
    pub original_size: usize,
    pub compressed_size: usize,
    pub compression_ratio: f64,
    pub error_message: Option<String>,
}

impl Default for Compressor {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;
    use tempfile::NamedTempFile;

    #[test]
    fn test_compress_decompress() {
        let compressor = Compressor::new();
        let original_data = b"Hello, World! This is a test message for compression. \
                             It should be long enough to see some compression benefits. \
                             Repeat: Hello, World! This is a test message for compression.";

        let compressed = compressor.compress(original_data).unwrap();
        let decompressed = compressor.decompress(&compressed).unwrap();

        assert_eq!(original_data.to_vec(), decompressed);
        assert!(compressed.len() < original_data.len()); // 应该有压缩效果
    }

    #[test]
    fn test_compress_empty_data() {
        let compressor = Compressor::new();
        let empty_data = b"";

        let compressed = compressor.compress(empty_data).unwrap();
        let decompressed = compressor.decompress(&compressed).unwrap();

        assert_eq!(empty_data.to_vec(), decompressed);
    }

    #[test]
    fn test_compress_file() {
        let compressor = Compressor::new();

        // 创建临时文件
        let mut temp_file = NamedTempFile::new().unwrap();
        let test_content = b"This is test file content for compression testing. \
                            It contains repeated patterns that should compress well. \
                            Repeated patterns, repeated patterns, repeated patterns.";
        temp_file.write_all(test_content).unwrap();

        let file_path = temp_file.path().to_str().unwrap();

        // 压缩文件
        let compressed_data = compressor.compress_file(file_path).unwrap();

        // 解压缩数据
        let decompressed_data = compressor.decompress(&compressed_data).unwrap();

        assert_eq!(test_content.to_vec(), decompressed_data);
    }

    #[test]
    fn test_compression_levels() {
        let original_data = b"Test data for compression level testing. \
                             This should be compressed with different levels. \
                             More text to make compression more effective.";

        let fast_compressor = Compressor::with_level(1);
        let best_compressor = Compressor::with_level(9);

        let fast_compressed = fast_compressor.compress(original_data).unwrap();
        let best_compressed = best_compressor.compress(original_data).unwrap();

        // 最佳压缩应该产生更小的结果
        assert!(best_compressed.len() <= fast_compressed.len());

        // 两种压缩都应该能正确解压
        let fast_decompressed = fast_compressor.decompress(&fast_compressed).unwrap();
        let best_decompressed = best_compressor.decompress(&best_compressed).unwrap();

        assert_eq!(original_data.to_vec(), fast_decompressed);
        assert_eq!(original_data.to_vec(), best_decompressed);
    }

    #[test]
    fn test_compress_with_stats() {
        let compressor = Compressor::new();
        let test_data = b"Test data for statistics. This should provide good compression stats.";

        let result = compressor.compress_with_stats(test_data).unwrap();

        assert_eq!(result.original_size, test_data.len());
        assert!(result.compressed_size > 0);
        assert!(result.compression_ratio >= 0.0);

        // 验证压缩数据可以正确解压
        let decompressed = compressor.decompress(&result.compressed_data).unwrap();
        assert_eq!(test_data.to_vec(), decompressed);
    }

    #[test]
    fn test_calculate_compression_ratio() {
        let compressor = Compressor::new();

        assert_eq!(compressor.calculate_compression_ratio(100, 50), 50.0);
        assert_eq!(compressor.calculate_compression_ratio(100, 75), 25.0);
        assert_eq!(compressor.calculate_compression_ratio(0, 0), 0.0);
        assert_eq!(compressor.calculate_compression_ratio(100, 100), 0.0);
    }

    #[test]
    fn test_stream_compression() {
        let compressor = Compressor::new();
        let test_data = b"Stream compression test data. This should work with streaming.";

        let compressed = compressor.compress_stream(test_data).unwrap();
        let decompressed = compressor.decompress_stream(&compressed).unwrap();

        assert_eq!(test_data.to_vec(), decompressed);
    }

    #[test]
    fn test_is_compressed() {
        let compressor = Compressor::new();
        let test_data = b"Test data for compression detection";

        let compressed = compressor.compress(test_data).unwrap();

        assert!(!compressor.is_compressed(test_data));
        assert!(compressor.is_compressed(&compressed));
    }

    #[test]
    fn test_adaptive_compress() {
        let compressor = Compressor::new();

        // 测试小数据
        let small_data = b"small";
        let small_compressed = compressor.adaptive_compress(small_data).unwrap();
        let small_decompressed = compressor.decompress(&small_compressed).unwrap();
        assert_eq!(small_data.to_vec(), small_decompressed);

        // 测试中等数据
        let medium_data = vec![b'x'; 5000];
        let medium_compressed = compressor.adaptive_compress(&medium_data).unwrap();
        let medium_decompressed = compressor.decompress(&medium_compressed).unwrap();
        assert_eq!(medium_data, medium_decompressed);
    }
}
