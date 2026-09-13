import 'package:dio/dio.dart';

/// Refreshes the access token. Returns the new token, or null if the session
/// is gone and the user has to sign in again.
typedef TokenRefresher = Future<String?> Function();

/// Attaches the bearer token to every request and transparently refreshes it
/// when the server says it has expired.
///
/// It extends [QueuedInterceptor] rather than [Interceptor] deliberately:
/// queued interceptors run one at a time, so ten screens firing requests at
/// once on a stale token produce exactly one refresh, and the other nine wait
/// for it instead of racing to rotate the refresh token — which, because
/// refresh tokens rotate on use, would log the user out.
class AuthInterceptor extends QueuedInterceptor {
  AuthInterceptor({
    required this.accessToken,
    required this.refresh,
    required this.onSessionLost,
    required Dio retryClient,
  }) : _retryClient = retryClient;

  /// Reads the current access token. Null before sign-in.
  final String? Function() accessToken;

  final TokenRefresher refresh;

  /// Called when refreshing failed: the session is unrecoverable.
  final void Function() onSessionLost;

  final Dio _retryClient;

  /// Requests that must never carry a bearer token or trigger a refresh,
  /// or a failed refresh would recurse into itself.
  static const _unauthenticatedPaths = <String>{
    '/api/v1/auth/login',
    '/api/v1/auth/refresh',
    '/api/v1/auth/logout',
    '/api/v1/ping',
    '/healthz',
  };

  static bool _isUnauthenticated(RequestOptions options) =>
      _unauthenticatedPaths.contains(Uri.parse(options.path).path) ||
      _unauthenticatedPaths.any(options.path.endsWith);

  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    final token = accessToken();
    if (token != null && !_isUnauthenticated(options)) {
      options.headers['Authorization'] = 'Bearer $token';
    }
    handler.next(options);
  }

  @override
  void onResponse(
    Response<dynamic> response,
    ResponseInterceptorHandler handler,
  ) async {
    // The client treats non-2xx as a normal response (so error envelopes can
    // be parsed), which means the 401 handling lives here rather than in
    // onError.
    if (response.statusCode != 401 ||
        _isUnauthenticated(response.requestOptions) ||
        response.requestOptions.extra.containsKey(_retriedKey)) {
      return handler.next(response);
    }

    final token = await refresh();
    if (token == null) {
      onSessionLost();
      return handler.next(response);
    }

    try {
      final retried = await _retryClient.fetch<dynamic>(
        response.requestOptions
          ..headers['Authorization'] = 'Bearer $token'
          ..extra[_retriedKey] = true,
      );
      handler.resolve(retried);
    } on DioException catch (e) {
      if (e.response != null) {
        handler.resolve(e.response!);
      } else {
        handler.next(response);
      }
    }
  }

  /// Marks a request as already retried, so a second 401 signs out instead of
  /// looping.
  static const _retriedKey = 'attic_retried_after_refresh';
}
