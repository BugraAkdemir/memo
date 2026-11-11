cmake_minimum_required(VERSION 3.15)
project(memo_notification_icon_tests LANGUAGES CXX)
add_executable(notification_icon_test notification_icon_test.cpp)
target_compile_features(notification_icon_test PRIVATE cxx_std_17)
target_link_libraries(notification_icon_test PRIVATE shell32 user32)
enable_testing()
add_test(NAME notification_icon_failures_and_recovery
  COMMAND notification_icon_test "${CMAKE_CURRENT_SOURCE_DIR}/../../../../lib/icon/memo.ico")
