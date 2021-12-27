#!/usr/bin/env bash

CERTIFICATE=${WINDOWS_SIGNING_CERTIFICATE}
PRIVATE_KEY=${WINDOWS_SIGNING_PRIVATE_KEY}
WEBSITE_URL=https://github.com/saucelabs/saucectl/
PROGRAM_DESCRIPTION=saucectl cli

UNIQUENESS=${echo ${RANDOM} | md5sum | cut -b 1-10}
CRT_PATH=/tmp/certificate-${UNIQUENESS}.crt
KEY_PATH=/tmp/private-${UNIQUENESS}.key
TMP_BINARY=${BINARY/saucectl.exe/saucectl-signed.exe}

if [[ "${TARGET}" == windows* ]];then
  echo "${CERTIFICATE_CONTENT}" | base64 -d > ${CRT_PATH}
  echo "${PRIVATE_KEY}" | base64 -d > ${KEY_PATH}

  osslsigncode sign \
    -certs ${CRT_PATH} \
    -key ${KEY_PATH} \
    -n ${PROGRAM_NAME} \
    -i  ${WEBSITE_URL} \
    -in ${BINARY} \
    -out ${TMP_BINARY}

  # Replace un-signed binary with signed binary
  mv -f ${TMP_BINARY} ${BINARY}

  # check Signature
  osslsigncode verify -CAfile ${CRT_PATH} ${BINARY}

  # Removing certificate
  rm -f ${CRT_PATH}
  rm -f ${KEY_PATH}
fi
