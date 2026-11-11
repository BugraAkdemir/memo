// SPDX-License-Identifier: MIT
// Memo patch to tray_manager 0.2.4. Upstream license retained in the package.
#ifndef MEMO_NOTIFICATION_ICON_H_
#define MEMO_NOTIFICATION_ICON_H_

#include <windows.h>
#include <shellapi.h>
#include <strsafe.h>

// Own the image and fully initialized shell identity. The injectable shell
// function permits deterministic failure/restart tests without a live Explorer.
class NotificationIcon {
 public:
  using Notify = decltype(&Shell_NotifyIconW);
  explicit NotificationIcon(Notify notify = &Shell_NotifyIconW)
      : notify_(notify) {}
  ~NotificationIcon() { Remove(); }
  NotificationIcon(const NotificationIcon&) = delete;
  NotificationIcon& operator=(const NotificationIcon&) = delete;

  bool Set(HWND window, UINT callback, const wchar_t* path) {
    HICON image = static_cast<HICON>(LoadImageW(
        nullptr, path, IMAGE_ICON, GetSystemMetrics(SM_CXSMICON),
        GetSystemMetrics(SM_CYSMICON), LR_LOADFROMFILE));
    if (!image) return false;
    const HICON old_image = data_.hIcon;
    data_.cbSize = sizeof(data_);
    data_.hWnd = window;
    data_.uID = 1;
    data_.uCallbackMessage = callback;
    data_.uFlags = NIF_MESSAGE | NIF_ICON;
    if (data_.szTip[0]) data_.uFlags |= NIF_TIP;
    data_.hIcon = image;
    const bool success = Apply();
    if (old_image) DestroyIcon(old_image);
    return success;
  }

  bool Restore() {
    // Explorer lost all registrations but our image and identity remain valid.
    registered_ = false;
    return data_.hIcon && Apply();
  }

  bool SetToolTip(const wchar_t* text) {
    if (!data_.hIcon) return false;
    StringCchCopyW(data_.szTip, _countof(data_.szTip), text);
    data_.uFlags |= NIF_TIP;
    return Apply();
  }

  bool GetBounds(RECT* bounds) const {
    NOTIFYICONIDENTIFIER identity{};
    identity.cbSize = sizeof(identity);
    identity.hWnd = data_.hWnd;
    identity.uID = data_.uID;
    return registered_ &&
           SUCCEEDED(Shell_NotifyIconGetRect(&identity, bounds));
  }

  bool has_image() const { return data_.hIcon != nullptr; }

  void Remove() {
    // Delete even after a failed MODIFY/ADD attempt; any prior registration
    // belongs to this same fixed identity. Cleanup is idempotent.
    if (data_.hWnd) notify_(NIM_DELETE, &data_);
    if (data_.hIcon) DestroyIcon(data_.hIcon);
    data_ = {};
    registered_ = false;
  }

 private:
  bool Apply() {
    if (registered_ && notify_(NIM_MODIFY, &data_)) return true;
    // A cached registration can be stale after Explorer loss. Try ADD rather
    // than reporting success based solely on the cached flag.
    registered_ = notify_(NIM_ADD, &data_) != FALSE;
    return registered_;
  }

  Notify notify_;
  NOTIFYICONDATAW data_{};
  bool registered_ = false;
};
#endif  // MEMO_NOTIFICATION_ICON_H_
