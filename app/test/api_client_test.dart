import 'package:attic/core/api/api_client.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('ApiClient', () {
    test('normalises a bare hostname into an https base URL', () {
      final client = ApiClient()..baseUrl = 'homeserver.tailnet.ts.net/';
      expect(client.baseUrl, 'https://homeserver.tailnet.ts.net');
    });

    test('keeps an explicit scheme and strips trailing slashes', () {
      final client = ApiClient()..baseUrl = 'http://192.168.1.10:8080//';
      expect(client.baseUrl, 'http://192.168.1.10:8080');
    });

    test('reports no base URL until one is set', () {
      expect(ApiClient().baseUrl, isNull);
    });
  });

  group('ApiError', () {
    test('parses the canonical error envelope', () {
      final response = Response<dynamic>(
        requestOptions: RequestOptions(path: '/api/v1/assets'),
        statusCode: 403,
        data: <String, dynamic>{
          'error': <String, dynamic>{
            'code': 'forbidden',
            'message': 'Not your asset.',
          },
        },
      );

      final error = ApiError.fromResponse(response);
      expect(error.code, 'forbidden');
      expect(error.message, 'Not your asset.');
      expect(error.status, 403);
    });

    test('degrades gracefully on a non-envelope body', () {
      final response = Response<dynamic>(
        requestOptions: RequestOptions(path: '/api/v1/assets'),
        statusCode: 502,
        data: '<html>bad gateway</html>',
      );

      final error = ApiError.fromResponse(response);
      expect(error.code, 'unknown');
      expect(error.status, 502);
    });
  });
}
