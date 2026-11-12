#include "../notification_icon.h"

#include <cstdint>
#include <iostream>
#include <stdexcept>
#include <string>
#include <vector>

namespace {
std::vector<DWORD> operations;
NOTIFYICONDATAW last{};
bool allow_add = true;
bool allow_modify = true;
BOOL WINAPI Notify(DWORD operation, PNOTIFYICONDATAW data) {
  operations.push_back(operation);
  last = *data;
  if (operation == NIM_ADD) return allow_add;
  if (operation == NIM_MODIFY) return allow_modify;
  return TRUE;
}
void Check(bool condition, const char* message) {
  if (!condition) throw std::runtime_error(message);
}
}  // namespace

int wmain(int argc, wchar_t** argv) {
  if (argc != 2) return 2;
  try {
    NotificationIcon icon(&Notify);
    HWND window = reinterpret_cast<HWND>(static_cast<uintptr_t>(0x1234));
    icon.Remove();
    Check(operations.empty(), "empty cleanup must not call the shell");
    Check(!icon.SetToolTip(L"Memo"), "tooltip without an icon must fail");
    Check(!icon.Set(window, WM_USER + 1, L"missing-memo-icon.ico"),
          "missing image must fail instead of reporting success");
    Check(operations.empty(), "missing image must not register a blank icon");

    allow_add = false;
    Check(!icon.Set(window, WM_USER + 1, argv[1]),
          "shell ADD failure must reach the caller");
    Check(icon.has_image(), "retain image for shell recovery");
    RECT bounds{};
    Check(!icon.GetBounds(&bounds), "failed registration has no bounds");
    Check(last.cbSize == sizeof(last) && last.hWnd == window && last.uID == 1,
          "shell identity must be initialized and stable");
    Check(last.hIcon && last.uCallbackMessage == WM_USER + 1,
          "image and callback must be set before ADD");
    const GUID zero_guid{};
    Check(last.uFlags == (NIF_ICON | NIF_MESSAGE) && last.szTip[0] == 0 &&
              last.dwState == 0 && last.dwStateMask == 0 &&
              last.szInfo[0] == 0 && last.uVersion == 0 &&
              IsEqualGUID(last.guidItem, zero_guid),
          "unused notification fields must not contain indeterminate data");

    allow_add = true;
    Check(icon.Restore(), "failed initialization must recover with ADD");
    Check(operations.back() == NIM_ADD, "recovery must register with Explorer");
    Check(icon.SetToolTip(L"Memo"), "tooltip update should succeed");
    Check(operations.back() == NIM_MODIFY && (last.uFlags & NIF_TIP),
          "tooltip update must modify existing identity");
    const HICON image = last.hIcon;
    Check(icon.Restore(), "Explorer restart must recover with ADD");
    Check(operations.back() == NIM_ADD && last.hIcon == image &&
              std::wstring(last.szTip) == L"Memo",
          "Explorer restart must retain callback image and tooltip");

    allow_modify = false;
    operations.clear();
    Check(icon.Set(window, WM_USER + 1, argv[1]),
          "stale MODIFY must fall back to successful ADD");
    Check(operations == std::vector<DWORD>({NIM_MODIFY, NIM_ADD}),
          "retry stale registration exactly once with ADD");
    allow_add = false;
    Check(!icon.Restore(), "failed Explorer restoration must reach caller");
    Check(!icon.GetBounds(&bounds), "lost registration must not expose bounds");
    allow_add = true;
    Check(icon.Restore(), "later Explorer restoration must remain possible");

    operations.clear();
    icon.Remove();
    icon.Remove();
    Check(!icon.has_image() && operations == std::vector<DWORD>({NIM_DELETE}),
          "cleanup must release once and be idempotent");
    std::cout << "PASS: missing image, shell failure, initialized identity, "
                 "retry, tooltip, Explorer recovery and cleanup\n";
    return 0;
  } catch (const std::exception& error) {
    std::cerr << "FAIL: " << error.what() << '\n';
    return 1;
  }
}
