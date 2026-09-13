import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';

import '../api/api_client.dart';
import '../api/api_error.dart';
import '../api/auth_interceptor.dart';
import 'token_store.dart';

/// The signed-in account.
class Account {
  const Account({required this.id, required this.username, required this.role});

  final String id;
  final String username;
  final String role;

  bool get isAdmin => role == 'admin';

  factory Account.fromJson(Map<String, dynamic> json) => Account(
        id: json['id'] as String? ?? '',
        username: json['username'] as String? ?? '',
        role: json['role'] as String? ?? 'member',
      );
}

/// Owns the session: the server address, the token pair, and the account.
///
/// Everything that talks to the server goes through the [ApiClient] this owns,
/// so there is exactly one place where tokens are attached and refreshed.
class AuthRepository {
  AuthRepository({required this.client, required this.store}) {
    client.addInterceptor(AuthInterceptor(
      accessToken: () => _accessToken,
      refresh: _refreshTokens,
      onSessionLost: () => _onSessionLost?.call(),
      // A separate client for the retry, with no interceptors of its own: the
      // retry must not be able to trigger another refresh.
      retryClient: _retryClient,
    ));
  }

  final ApiClient client;
  final SessionStore store;

  late final Dio _retryClient = Dio(client.dio.options);

  /// The bare client used for refreshes and replays. Exposed so tests can
  /// point it at the same fake transport as the main client.
  @visibleForTesting
  Dio get dioRetryClient => _retryClient;

  String? _accessToken;
  String? _refreshToken;
  void Function()? _onSessionLost;

  /// Registers the callback fired when the session cannot be refreshed.
  set onSessionLost(void Function()? callback) => _onSessionLost = callback;

  String? get serverUrl => client.baseUrl;
  bool get hasSession => _accessToken != null;

  /// The media token used for `?token=` URLs. Players cannot refresh a header
  /// mid-stream, so playback URLs carry one of these instead.
  String? _mediaToken;
  String? get mediaToken => _mediaToken;

  /// Restores a session saved by a previous launch.
  Future<StoredSession?> restore() async {
    final session = await store.read();
    if (session == null) {
      final server = await store.readServerUrl();
      if (server != null) client.baseUrl = server;
      return null;
    }

    client.baseUrl = session.serverUrl;
    _accessToken = session.accessToken;
    _refreshToken = session.refreshToken;
    return session;
  }

  /// Checks that an address is actually an Attic server, and remembers it.
  ///
  /// `/healthz` rather than an API route: it is unauthenticated and it is the
  /// same endpoint the container healthcheck uses, so if this succeeds the
  /// server is genuinely up rather than merely reachable.
  Future<void> connect(String address) async {
    final normalized = ApiClient.normalizeServerUrl(address);
    final probe = Dio(BaseOptions(
      baseUrl: normalized,
      connectTimeout: const Duration(seconds: 8),
      receiveTimeout: const Duration(seconds: 8),
      validateStatus: (_) => true,
    ));

    late final Response<dynamic> response;
    try {
      response = await probe.get<dynamic>('/healthz');
    } on DioException catch (e) {
      throw ApiError.fromDio(e);
    }

    final body = response.data;
    if (response.statusCode != 200 || body is! Map || body['status'] != 'ok') {
      throw const ApiError(
        code: 'not_attic',
        message: 'That address answered, but it is not an Attic server.',
      );
    }

    client.baseUrl = normalized;
    await store.writeServerUrl(normalized);
  }

  /// Exchanges credentials for a token pair bound to this device.
  Future<Account> signIn({
    required String username,
    required String password,
    required String deviceName,
  }) async {
    final json = await client.postJson('/auth/login', body: {
      'username': username,
      'password': password,
      'device_name': deviceName,
    });

    await _adopt(json);
    return Account.fromJson(json['user'] as Map<String, dynamic>? ?? const {});
  }

  /// Loads the signed-in account.
  Future<Account> me() async =>
      Account.fromJson(await client.getJson('/me'));

  /// Revokes this device's session on the server, then locally.
  Future<void> signOut() async {
    final refresh = _refreshToken;
    _accessToken = null;
    _refreshToken = null;
    _mediaToken = null;

    if (refresh != null) {
      try {
        await client.postJson('/auth/logout', body: {'refresh_token': refresh});
      } on ApiError {
        // The session is already gone as far as this device is concerned; a
        // server that cannot be reached must not trap someone in a signed-in
        // state.
      }
    }
    await store.clearSession();
  }

  /// Fetches a media token for `?token=` playback URLs.
  Future<String?> ensureMediaToken({bool force = false}) async {
    if (_mediaToken != null && !force) return _mediaToken;
    try {
      final json = await client.postJson('/auth/media-token');
      _mediaToken = json['token'] as String?;
    } on ApiError {
      _mediaToken = null;
    }
    return _mediaToken;
  }

  /// Signs a media URL so a player can fetch it without headers.
  String signMediaUrl(String path) {
    final url = client.resolve(path);
    final token = _mediaToken;
    if (token == null) return url;
    final separator = url.contains('?') ? '&' : '?';
    return '$url$separator' 'token=${Uri.encodeQueryComponent(token)}';
  }

  /// Rotates the token pair. Returns the new access token, or null when the
  /// session is unrecoverable.
  Future<String?> _refreshTokens() async {
    final refresh = _refreshToken;
    if (refresh == null) return null;

    try {
      // The bare client: a refresh must not pass back through the interceptor
      // that is waiting on it.
      final response = await _retryClient.post<dynamic>(
        '/api/v1/auth/refresh',
        data: {'refresh_token': refresh},
      );
      final body = response.data;
      if (response.statusCode != 200 || body is! Map<String, dynamic>) {
        await _forget();
        return null;
      }
      await _adopt(body);
      // Media tokens are minted from the access token, so a refreshed session
      // needs a fresh one before the next track loads.
      _mediaToken = null;
      return _accessToken;
    } on DioException {
      // A network failure is not a revoked session: keep the tokens so the
      // next attempt can succeed once the tailnet is back.
      return null;
    }
  }

  Future<void> _adopt(Map<String, dynamic> json) async {
    _accessToken = json['access_token'] as String?;
    _refreshToken = json['refresh_token'] as String?;

    final server = client.baseUrl;
    if (server != null && _accessToken != null && _refreshToken != null) {
      await store.write(StoredSession(
        serverUrl: server,
        accessToken: _accessToken!,
        refreshToken: _refreshToken!,
      ));
    }
  }

  Future<void> _forget() async {
    _accessToken = null;
    _refreshToken = null;
    _mediaToken = null;
    await store.clearSession();
  }
}
