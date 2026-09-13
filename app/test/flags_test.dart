import 'package:attic/core/flags.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('every unfinished feature ships off by default', () {
    // Guards the hard rule: a default build of main is always demoable.
    expect(Flags.auth, isFalse);
    expect(Flags.photos, isFalse);
    expect(Flags.music, isFalse);
    expect(Flags.video, isFalse);
    expect(Flags.backgroundSync, isFalse);
  });
}
