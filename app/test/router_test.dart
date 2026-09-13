import 'package:attic/core/router/app_router.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('destinationIndexFor', () {
    test('maps each top-level route to its destination', () {
      expect(destinationIndexFor('/photos'), 0);
      expect(destinationIndexFor('/music'), 1);
      expect(destinationIndexFor('/video'), 2);
      expect(destinationIndexFor('/settings'), 3);
    });

    test('keeps the parent destination selected on nested routes', () {
      expect(destinationIndexFor('/music/album/42'), 1);
    });

    test('falls back to the first destination for unknown routes', () {
      expect(destinationIndexFor('/nowhere'), 0);
    });
  });
}
