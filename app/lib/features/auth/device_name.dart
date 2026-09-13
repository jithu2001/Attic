import 'dart:io';

/// A human-readable name for this device, shown in the account's device list
/// so a session can be recognised — and revoked — later.
Future<String> describeDevice() async {
  try {
    final host = Platform.localHostname;
    if (host.isNotEmpty && host != 'localhost') return host;
  } on Object {
    // Platform.localHostname throws on some sandboxed platforms.
  }

  if (Platform.isAndroid) return 'Android device';
  if (Platform.isIOS) return 'iPhone';
  return 'Attic client';
}
