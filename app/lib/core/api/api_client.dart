import 'package:dio/dio.dart';

/// Canonical API error envelope: `{"error": {"code": ..., "message": ...}}`.
class ApiError implements Exception {
  const ApiError({required this.code, required this.message, this.status});

  final String code;
  final String message;
  final int? status;

  factory ApiError.fromResponse(Response<dynamic> response) {
    final data = response.data;
    if (data is Map && data['error'] is Map) {
      final error = data['error'] as Map;
      return ApiError(
        code: error['code']?.toString() ?? 'unknown',
        message: error['message']?.toString() ?? 'Request failed.',
        status: response.statusCode,
      );
    }
    return ApiError(
      code: 'unknown',
      message: 'Request failed (HTTP ${response.statusCode}).',
      status: response.statusCode,
    );
  }

  @override
  String toString() => 'ApiError($code): $message';
}

/// Thin wrapper over dio that knows Attic's conventions: a `/api/v1` base path
/// on the configured server, JSON in and out, and the error envelope above.
///
/// The auth interceptor (bearer access token + refresh-on-401) is added in the
/// phase that ships authentication; the hook is [attachInterceptor].
class ApiClient {
  ApiClient({Dio? dio}) : _dio = dio ?? Dio() {
    _dio.options
      ..connectTimeout = const Duration(seconds: 10)
      ..receiveTimeout = const Duration(seconds: 30)
      ..headers['Accept'] = 'application/json'
      // Let non-2xx responses through so they can be turned into ApiError
      // rather than an opaque DioException.
      ..validateStatus = (_) => true;
  }

  final Dio _dio;

  /// The server's base URL, e.g. `https://homeserver.tailnet.ts.net`.
  /// Attic has no public domain; this always points inside the tailnet.
  String? get baseUrl => _dio.options.baseUrl.isEmpty ? null : _dio.options.baseUrl;

  set baseUrl(String? value) {
    _dio.options.baseUrl = value == null ? '' : _normalize(value);
  }

  void attachInterceptor(Interceptor interceptor) =>
      _dio.interceptors.add(interceptor);

  /// GET a JSON object from a path relative to `/api/v1`.
  Future<Map<String, dynamic>> getJson(
    String path, {
    Map<String, dynamic>? query,
  }) async {
    final response = await _dio.get<dynamic>(
      _apiPath(path),
      queryParameters: query,
    );
    return _unwrap(response);
  }

  /// POST a JSON body to a path relative to `/api/v1`.
  Future<Map<String, dynamic>> postJson(
    String path, {
    Object? body,
  }) async {
    final response = await _dio.post<dynamic>(_apiPath(path), data: body);
    return _unwrap(response);
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

  static String _normalize(String url) {
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
