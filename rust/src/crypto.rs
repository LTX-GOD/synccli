use aes_gcm::{
    Aes256Gcm, Key, Nonce,
    aead::{Aead, AeadCore, KeyInit, OsRng},
};
use sha2::{Digest, Sha256};
use std::fs;
use std::io::Read;

/// 加密压缩器
pub struct CryptoCompressor {
    // 可以添加配置选项
}

impl CryptoCompressor {
    /// 创建新的加密压缩器
    pub fn new() -> Self {
        Self {}
    }

    /// 从密码生成密钥
    fn derive_key_from_password(&self, password: &[u8]) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(password);
        hasher.update(b"synccli-salt"); // 添加盐值
        let result = hasher.finalize();
        let mut key = [0u8; 32];
        key.copy_from_slice(&result);
        key
    }

    /// 加密数据
    pub fn encrypt_data(&self, data: &[u8], password: &[u8]) -> Result<Vec<u8>, String> {
        // 从密码派生密钥
        let key_bytes = self.derive_key_from_password(password);
        let key = Key::<Aes256Gcm>::from_slice(&key_bytes);

        // 创建加密器
        let cipher = Aes256Gcm::new(key);

        // 生成随机nonce
        let nonce = Aes256Gcm::generate_nonce(&mut OsRng);

        // 加密数据
        match cipher.encrypt(&nonce, data) {
            Ok(ciphertext) => {
                // 将nonce和密文组合
                let mut result = Vec::new();
                result.extend_from_slice(&nonce);
                result.extend_from_slice(&ciphertext);
                Ok(result)
            }
            Err(e) => Err(format!("加密失败: {}", e)),
        }
    }

    /// 解密数据
    pub fn decrypt_data(&self, encrypted_data: &[u8], password: &[u8]) -> Result<Vec<u8>, String> {
        if encrypted_data.len() < 12 {
            return Err("加密数据太短".to_string());
        }

        // 从密码派生密钥
        let key_bytes = self.derive_key_from_password(password);
        let key = Key::<Aes256Gcm>::from_slice(&key_bytes);

        // 创建解密器
        let cipher = Aes256Gcm::new(key);

        // 提取nonce和密文
        let (nonce_bytes, ciphertext) = encrypted_data.split_at(12);
        let nonce = Nonce::from_slice(nonce_bytes);

        // 解密数据
        match cipher.decrypt(nonce, ciphertext) {
            Ok(plaintext) => Ok(plaintext),
            Err(e) => Err(format!("解密失败: {}", e)),
        }
    }

    /// 加密文件
    pub fn encrypt_file(&self, file_path: &str, password: &[u8]) -> Result<Vec<u8>, String> {
        // 读取文件内容
        let file_data =
            fs::read(file_path).map_err(|e| format!("读取文件失败 {}: {}", file_path, e))?;

        // 加密数据
        self.encrypt_data(&file_data, password)
    }

    /// 解密文件并保存
    pub fn decrypt_file_to_path(
        &self,
        encrypted_data: &[u8],
        password: &[u8],
        output_path: &str,
    ) -> Result<(), String> {
        // 解密数据
        let decrypted_data = self.decrypt_data(encrypted_data, password)?;

        // 写入文件
        fs::write(output_path, decrypted_data)
            .map_err(|e| format!("写入文件失败 {}: {}", output_path, e))?;

        Ok(())
    }

    /// 加密文件流（用于大文件）
    pub fn encrypt_file_stream(
        &self,
        file_path: &str,
        password: &[u8],
        chunk_size: usize,
    ) -> Result<Vec<u8>, String> {
        let mut file =
            fs::File::open(file_path).map_err(|e| format!("打开文件失败 {}: {}", file_path, e))?;

        let mut buffer = vec![0u8; chunk_size];
        let mut all_data = Vec::new();

        loop {
            match file.read(&mut buffer) {
                Ok(0) => break, // 文件结束
                Ok(n) => {
                    all_data.extend_from_slice(&buffer[..n]);
                }
                Err(e) => return Err(format!("读取文件失败: {}", e)),
            }
        }

        self.encrypt_data(&all_data, password)
    }

    /// 验证密码是否正确
    pub fn verify_password(&self, encrypted_data: &[u8], password: &[u8]) -> bool {
        self.decrypt_data(encrypted_data, password).is_ok()
    }

    /// 生成随机密码
    pub fn generate_random_password(&self, length: usize) -> String {
        use rand::Rng;
        const CHARSET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZ\
                                abcdefghijklmnopqrstuvwxyz\
                                0123456789\
                                !@#$%^&*";

        let mut rng = rand::thread_rng();
        (0..length)
            .map(|_| {
                let idx = rng.gen_range(0..CHARSET.len());
                CHARSET[idx] as char
            })
            .collect()
    }

    /// 计算数据的哈希值（用于完整性验证）
    pub fn calculate_hash(&self, data: &[u8]) -> String {
        let mut hasher = Sha256::new();
        hasher.update(data);
        let result = hasher.finalize();
        hex::encode(result)
    }

    /// 加密并计算哈希
    pub fn encrypt_with_hash(
        &self,
        data: &[u8],
        password: &[u8],
    ) -> Result<(Vec<u8>, String), String> {
        let encrypted_data = self.encrypt_data(data, password)?;
        let hash = self.calculate_hash(&encrypted_data);
        Ok((encrypted_data, hash))
    }

    /// 解密并验证哈希
    pub fn decrypt_with_hash_verification(
        &self,
        encrypted_data: &[u8],
        password: &[u8],
        expected_hash: &str,
    ) -> Result<Vec<u8>, String> {
        // 验证哈希
        let actual_hash = self.calculate_hash(encrypted_data);
        if actual_hash != expected_hash {
            return Err("数据完整性验证失败".to_string());
        }

        // 解密数据
        self.decrypt_data(encrypted_data, password)
    }
}

impl Default for CryptoCompressor {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use std::io::Write;
    use tempfile::NamedTempFile;

    #[test]
    fn test_encrypt_decrypt_data() {
        let crypto = CryptoCompressor::new();
        let data = b"Hello, World! This is a test message.";
        let password = b"test_password_123";

        let encrypted = crypto.encrypt_data(data, password).unwrap();
        let decrypted = crypto.decrypt_data(&encrypted, password).unwrap();

        assert_eq!(data.to_vec(), decrypted);
    }

    #[test]
    fn test_encrypt_decrypt_with_wrong_password() {
        let crypto = CryptoCompressor::new();
        let data = b"Secret message";
        let password = b"correct_password";
        let wrong_password = b"wrong_password";

        let encrypted = crypto.encrypt_data(data, password).unwrap();
        let result = crypto.decrypt_data(&encrypted, wrong_password);

        assert!(result.is_err());
    }

    #[test]
    fn test_encrypt_decrypt_file() {
        let crypto = CryptoCompressor::new();
        let password = b"file_password_123";

        // 创建临时文件
        let mut temp_file = NamedTempFile::new().unwrap();
        let test_content = b"This is test file content for encryption.";
        temp_file.write_all(test_content).unwrap();

        let file_path = temp_file.path().to_str().unwrap();

        // 加密文件
        let encrypted_data = crypto.encrypt_file(file_path, password).unwrap();

        // 解密数据
        let decrypted_data = crypto.decrypt_data(&encrypted_data, password).unwrap();

        assert_eq!(test_content.to_vec(), decrypted_data);
    }

    #[test]
    fn test_verify_password() {
        let crypto = CryptoCompressor::new();
        let data = b"Test data for password verification";
        let correct_password = b"correct123";
        let wrong_password = b"wrong123";

        let encrypted = crypto.encrypt_data(data, correct_password).unwrap();

        assert!(crypto.verify_password(&encrypted, correct_password));
        assert!(!crypto.verify_password(&encrypted, wrong_password));
    }

    #[test]
    fn test_generate_random_password() {
        let crypto = CryptoCompressor::new();

        let password1 = crypto.generate_random_password(16);
        let password2 = crypto.generate_random_password(16);

        assert_eq!(password1.len(), 16);
        assert_eq!(password2.len(), 16);
        assert_ne!(password1, password2); // 应该生成不同的密码
    }

    #[test]
    fn test_calculate_hash() {
        let crypto = CryptoCompressor::new();
        let data1 = b"test data";
        let data2 = b"test data";
        let data3 = b"different data";

        let hash1 = crypto.calculate_hash(data1);
        let hash2 = crypto.calculate_hash(data2);
        let hash3 = crypto.calculate_hash(data3);

        assert_eq!(hash1, hash2); // 相同数据应该有相同哈希
        assert_ne!(hash1, hash3); // 不同数据应该有不同哈希
        assert_eq!(hash1.len(), 64); // SHA256 哈希长度为64个十六进制字符
    }

    #[test]
    fn test_encrypt_with_hash() {
        let crypto = CryptoCompressor::new();
        let data = b"Test data with hash";
        let password = b"test_password";

        let (encrypted_data, hash) = crypto.encrypt_with_hash(data, password).unwrap();
        let decrypted_data = crypto
            .decrypt_with_hash_verification(&encrypted_data, password, &hash)
            .unwrap();

        assert_eq!(data.to_vec(), decrypted_data);
    }

    #[test]
    fn test_decrypt_with_hash_verification_failure() {
        let crypto = CryptoCompressor::new();
        let data = b"Test data";
        let password = b"test_password";

        let (encrypted_data, _) = crypto.encrypt_with_hash(data, password).unwrap();
        let wrong_hash = "wrong_hash_value";

        let result = crypto.decrypt_with_hash_verification(&encrypted_data, password, wrong_hash);

        assert!(result.is_err());
        assert!(result.unwrap_err().contains("完整性验证失败"));
    }
}
