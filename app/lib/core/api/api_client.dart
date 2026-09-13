import 'package:dio/dio.dart';

import 'api_error.dart';

/// Thin wrapper over dio that knows Attic's conventions: a `/api/v1` base path
/// on the configured server, JSON in and out, and the canonical error envelope.
class ApiClient {
  ApiClient({Dio? dio}) : dio = dio ?? Dio() {
    this.dio.options
      ..connectTimeout = const Duration(seconds: 10)
      ..receiveTimeout = const Duration(seconds: 30)
      ..headers['Accept'] = 'application/json'
      // Let non-2xx responses through instead of throwing, so they can be
      // turned into a typed ApiError with the server's own message in it.
      ..validateStatus = (_) => true;
  }

  final Dio dio;

  /// The server's base URL, e.g. `https://homeserver.tailnet.ts.net`.
  /// Attic has no public domain; this always points inside a tailnet.
  String? get baseUrl => dio.options.baseUrl.isEmpty ? null : dio.options.baseUrl;

  set baseUrl(String? value) {
    dio.options.baseUrl = value == null ? '' : normalizeServerUrl(value);
  }

  void addInterceptor(Interceptor interceptor) => dio.interceptors.add(interceptor);

  /// Builds an absolute URL for a server-relative path such as the `audio_url`
  /// and `cover_url` the API hands out.
  String resolve(String path) {
    final base = baseUrl;
    if (base == null) return path;
    return path.startsWith('/') ? '$base$path' : '$base/$path';
  }

  Future<Map<String, dynamic>> getJson(
    String path, {
    Map<String, dynamic>? query,
  }) async {
    return _unwrap(await _send(() => dio.get<dynamic>(
          _apiPath(path),
          queryParameters: query,
        )));
  }

  Future<Map<String, dynamic>> postJson(String path, {Object? body}) async {
    return _unwrap(await _send(() => dio.post<dynamic>(_apiPath(path), data: body)));
  }

  Future<Map<String, dynamic>> putJson(String path, {Object? body}) async {
    return _unwrap(await _send(() => dio.put<dynamic>(_apiPath(path), data: body)));
  }

  Future<void> delete(String path) async {
    _unwrap(await _send(() => dio.delete<dynamic>(_apiPath(path))));
  }

  Future<Response<dynamic>> _send(Future<Response<dynamic>> Function() call) async {
    try {
      return await call();
    } on DioException catch (e) {
      throw ApiError.fromDio(e);
    }
  }

  Map<String, dynamic> _unwrap(Response<dynamic> response) {
    final status = response.statusCode ?? 0;
    if (status < 200 || status >= 300) {
      throw ApiError.fromResponse(response);
    }
    final data = response.data;
    if (data is Map<String, dynamic>) return data;
    return <String, dynamic>{};
  }

  static String _apiPath(String path) =>
      path.startsWith('/') ? '/api/v1$path' : '/api/v1/$path';

  /// Accepts what someone would actually type — `homeserver.tailnet.ts.net`,
  /// with or without a scheme or trailing slash — and produces a base URL.
  static String normalizeServerUrl(String url) {
    var normalized = url.trim();
    if (!normalized.startsWith('http://') && !normalized.startsWith('https://')) {
      normalized = 'https://$normalized';
    }
    while (normalized.endsWith('/')) {
      normalized = normalized.substring(0, normalized.length - 1);
    }
    return normalized;
  }
}
