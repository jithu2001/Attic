import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// A stored session: where the server is, and the credentials for it.
class StoredSession {
  const StoredSession({
    required this.serverUrl,
    required this.accessToken,
    required this.refreshToken,
  });

  final String serverUrl;
  final String accessToken;
  final String refreshToken;

  StoredSession copyWith({String? accessToken, String? refreshToken}) {
    return StoredSession(
      serverUrl: serverUrl,
      accessToken: accessToken ?? this.accessToken,
      refreshToken: refreshToken ?? this.refreshToken,
    );
  }
}

/// Where a session is persisted between launches.
///
/// An interface rather than a concrete class so the refresh logic can be tested
/// without a platform keystore.
abstract class SessionStore {
  Future<StoredSession?> read();
  Future<String?> readServerUrl();
  Future<void> writeServerUrl(String url);
  Future<void> write(StoredSession session);
  Future<void> writeTokens({
    required String accessToken,
    required String refreshToken,
  });
  Future<void> clearSession();
}

/// Persists tokens in the platform keystore.
///
/// Tokens never touch SharedPreferences: the refresh token is a long-lived
/// credential for someone's entire media library, so it belongs in the
/// Keychain / EncryptedSharedPreferences and nowhere else. The server address
/// is kept alongside them for simplicity.
class TokenStore implements SessionStore {
  TokenStore({FlutterSecureStorage? storage})
      : _storage = storage ??
            const FlutterSecureStorage(
              aOptions: AndroidOptions(encryptedSharedPreferences: true),
            );

  static const _serverKey = 'server_url';
  static const _accessKey = 'access_token';
  static const _refreshKey = 'refresh_token';

  final FlutterSecureStorage _storage;

  @override
  Future<StoredSession?> read() async {
    final values = await Future.wait([
      _storage.read(key: _serverKey),
      _storage.read(key: _accessKey),
      _storage.read(key: _refreshKey),
    ]);

    final server = values[0];
    final access = values[1];
    final refresh = values[2];
    if (server == null || access == null || refresh == null) return null;

    return StoredSession(
      serverUrl: server,
      accessToken: access,
      refreshToken: refresh,
    );
  }

  /// Reads just the server address, which outlives a session: signing out
  /// should not make someone retype their hostname.
  @override
  Future<String?> readServerUrl() => _storage.read(key: _serverKey);

  @override
  Future<void> writeServerUrl(String url) =>
      _storage.write(key: _serverKey, value: url);

  @override
  Future<void> write(StoredSession session) async {
    await Future.wait([
      _storage.write(key: _serverKey, value: session.serverUrl),
      _storage.write(key: _accessKey, value: session.accessToken),
      _storage.write(key: _refreshKey, value: session.refreshToken),
    ]);
  }

  @override
  Future<void> writeTokens({
    required String accessToken,
    required String refreshToken,
  }) async {
    await Future.wait([
      _storage.write(key: _accessKey, value: accessToken),
      _storage.write(key: _refreshKey, value: refreshToken),
    ]);
  }

  /// Clears the session but keeps the server address.
  @override
  Future<void> clearSession() async {
    await Future.wait([
      _storage.delete(key: _accessKey),
      _storage.delete(key: _refreshKey),
    ]);
  }
}
