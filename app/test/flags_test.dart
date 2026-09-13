import 'package:attic/core/flags.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('shipped features are on and unfinished ones are off', () {
    // Guards the hard rule: a default build of main is always demoable, and
    // nothing half-built is reachable. Auth and music shipped in phase 1.
    expect(Flags.auth, isTrue);
    expect(Flags.music, isTrue);

    expect(Flags.photos, isFalse);
    expect(Flags.video, isFalse);
    expect(Flags.backgroundSync, isFalse);
  });
}
