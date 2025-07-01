cmake_minimum_required(VERSION 3.10)
project(PSKConn LANGUAGES C CXX)

set(CMAKE_CXX_STANDARD 11)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Add source files here
set(PSKConn_SOURCES
    src/PSKConn.cpp
)

add_library(PSKConn STATIC ${PSKConn_SOURCES})
target_include_directories(PSKConn PUBLIC ${CMAKE_CURRENT_SOURCE_DIR})

# Find and link OpenSSL
find_package(OpenSSL REQUIRED)
target_link_libraries(PSKConn PUBLIC OpenSSL::SSL OpenSSL::Crypto)

# Copy built library to lib/<platform> directory
if (APPLE)
    set(PLATFORM_NAME "darwin")
    set(CMAKE_OSX_DEPLOYMENT_TARGET "14.0")
    set(CMAKE_OSX_ARCHITECTURES "x86_64;arm64")
elseif (WIN32)
    set(PLATFORM_NAME "windows")
elseif (UNIX)
    set(PLATFORM_NAME "linux")
else()
    set(PLATFORM_NAME "unknown")
endif()

set(OUTPUT_LIB_DIR "${CMAKE_CURRENT_SOURCE_DIR}/lib/${PLATFORM_NAME}")
file(MAKE_DIRECTORY ${OUTPUT_LIB_DIR})
add_custom_command(TARGET PSKConn POST_BUILD
    COMMAND ${CMAKE_COMMAND} -E copy $<TARGET_FILE:PSKConn> ${OUTPUT_LIB_DIR}
    COMMENT "Copying PSKConn library to ${OUTPUT_LIB_DIR}"
)
