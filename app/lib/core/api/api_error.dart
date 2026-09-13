import 'package:dio/dio.dart';

/// The canonical API error envelope: `{"error": {"code": ..., "message": ...}}`.
///
/// The code is stable and switchable; the message is written for people and is
/// what the UI shows in a SnackBar.
class ApiError implements Exception {
  const ApiError({required this.code, required this.message, this.status});

  final String code;
  final String message;
  final int? status;

  /// The session is gone and the user must sign in again.
  bool get isUnauthorized => status == 401;

  /// The access token expired but the session is probably still good.
  bool get isExpiredToken => code == 'token_expired';

  bool get isNotFound => status == 404;

  factory ApiError.fromResponse(Response<dynamic> response) {
    final data = response.data;
    if (data is Map && data['error'] is Map) {
      final error = data['error'] as Map;
      return ApiError(
        code: error['code']?.toString() ?? 'unknown',
        message: error['message']?.toString() ?? 'Something went wrong.',
        status: response.statusCode,
      );
    }
    return ApiError(
      code: 'unknown',
      message: 'The server returned an unexpected response '
          '(HTTP ${response.statusCode}).',
      status: response.statusCode,
    );
  }

  /// Turns a transport-level failure into a message worth showing. Attic is
  /// only reachable over the tailnet, so "offline" here usually means "not on
  /// the tailnet", which is the hint people actually need.
  factory ApiError.fromDio(DioException e) {
    if (e.response != null) return ApiError.fromResponse(e.response!);

    final message = switch (e.type) {
      DioExceptionType.connectionTimeout ||
      DioExceptionType.sendTimeout ||
      DioExceptionType.receiveTimeout =>
        'The server took too long to answer.',
      DioExceptionType.badCertificate =>
        'The server’s certificate could not be verified.',
      DioExceptionType.cancel => 'The request was cancelled.',
      _ => 'Could not reach your server. Check that you are connected to your '
          'tailnet.',
    };
    return ApiError(code: 'network', message: message);
  }

  @override
  String toString() => 'ApiError($code): $message';
}
