// internal/utils/hash.go
package utils

import "golang.org/x/crypto/bcrypt" // Paket Go standar (sub-repositori) untuk hashing password bcrypt.

// HashPassword menghasilkan hash bcrypt dari string password yang diberikan.
// Menggunakan cost default bcrypt untuk keseimbangan antara keamanan dan performa.
// Mengembalikan hash sebagai string atau error jika terjadi masalah saat hashing.
func HashPassword(password string) (string, error) {
	// GenerateFromPassword meng-hash password menggunakan salt acak (sudah termasuk dalam hash).
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		// Error jarang terjadi kecuali ada masalah sistem atau password terlalu panjang.
		return "", err // Kembalikan error asli
	}
	return string(bytes), nil // Kembalikan hash dalam bentuk string
}

// CheckPasswordHash membandingkan password plaintext dengan hash bcrypt yang sudah ada.
// Fungsi ini secara otomatis mengekstrak salt dari hash dan melakukan perbandingan yang aman.
// Mengembalikan true jika password cocok dengan hash, false jika tidak atau jika ada error.
func CheckPasswordHash(password, hash string) bool {
	// CompareHashAndPassword adalah cara yang aman untuk membandingkan, tahan terhadap timing attacks.
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	// Mengembalikan true hanya jika err adalah nil (tidak ada error, berarti cocok).
	return err == nil
}
