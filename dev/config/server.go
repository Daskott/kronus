package config

const SERVER_YML = `
kronus:
  privateKeyPem: "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDSrTqLNNFUr08X\nEzy5LlYXRqgDuSe0oGZs6R7wa8B60ZdlW7GEgG8DwR17pJH2QAaI4FjKwXhHXZs9\n2M/mSESnTne/IgJ3RWdT0R7nrM7NFknp2KqApVcDQSH5d5jZKVQHopwU2Es0vWsW\nbG45T0eBLZojCnWCjauTyPYJlSMeDrXOg6ngKM3WfMQMlL3cgk90hEtMnJ0ycDle\nAMTEYPm/CBqu/XC0Tr5z1DzQi0pC9RG/d4imt/7KqG6ZmUURS6pzDBG2aw6ipmeu\nBfcpu55g9mGERnYXIj/gSDmtg+JUeDsq58MsZ7ZT8YUyzoiY3z28VvXuYHLUNSX2\n1MkgfDiNAgMBAAECggEBAK4Dj7uz4MPGGdnBdgKvF0Uag2Sv5u/3HSMQWxHSrqXD\nwP1jg3kibI/5TtT11epEcCFWzYCL1UF9O+EV2IMpZiubUKV6/fZuSS6eKJzLy/Ty\nWBLjd9HSv9BcWCeqdYHJ9TJpSeqdzWC+pFldLp3/sdwtQod2+CDhy7rB3xeDLAKC\nPFkQP5nTAjQW4hzIZN3YueShbrFzyy9+coRRDREYfs8UsMWT1TJFjJpTppBpUARB\nh1wZJDAR/KMi5NZS6IzFLAXi3Ur+YdDitG/oz5k4LLRoyrbIma2CSZQ8+QDLYJaw\nmIUktSbb1jzbV1e7kSm+zn2PmWUqbQmYVnGkBkO8zTECgYEA6eljJHC+ets9FRiR\nehrIGMVKRJDDwskCfCRLanLoJB6+513iMOUrCnNa/THDUXn19KFe8mboWTgn7A5/\naJ+rGMa8dKegKSnZxyUlQngjkRBP7ozHDbFJnDF5Dfpwx3k8ok9XwCfF5gC+YzmQ\nkVirWeTCUsvmLNjRS9Fc5F2G6YsCgYEA5pInzRAjbJR87ONaq8nCbLkfkKRXO5ck\nhZ25IS36Eu/tfi2xBMxFImGqosgKAhDall216nrNVeic9feqUIS0v9pvapkc+OJG\nyPddSy+BNd6mpHR2/Nim6WEH0gagxATTN+a3tQbIrrTqgq4l17+M2q9HMnaLXRke\n4a9NhUVkuUcCgYBGSZA2Cf7i0fBH34sPYu7PqrEHa2y3okkx3oIe6YpiGC8LPQXT\n5XkKeeFUhdiIKhrDOJ5cPpoA/UPZxf15BcmW91j3wMr6s42yLrJEh+9ADuPF7d1+\netCAs8kJb0DmX8LdjvPyVME9vOl4zXpognly2K+fy49N2JUDsFS2dngswwKBgCKB\nnRNDZwnI7ylEnT04ZLCAxAiRj7yLUhvtDte4WcSbw58ul19wcqhClZbm+Rh2DUCT\npbYBytkght0Iw6RpN+O+fQ4m+/8DXjSVUJD/+wZk2+ugwm30voYOz2zPMSAk2Ld0\n/+lHqqD60l3cUi2HrTzNHoqe0xyLteNwqNlZGUnhAoGAJDrUxiDPLVcvctgiL0/1\nfIdXdaz61ISM/MzbNOlplW4uoejnTk4JLXCPM7JH/Mf1vr7A/WCBQy7k4HK7Yxnh\n+1HcFSADL+yILhEQYwc366NI/phOMbXMxUNlp5QYKPTlbxpr0LQeamIaLi+9nXkF\nkKeZzVA0xAVgPkqQ+FBM7rA=\n-----END PRIVATE KEY-----\n"
  cron:
    timeZone: "America/Toronto"
  listener:
    port: 3000

sqlite:
  passPhrase: passphrase

google:
  storage:
    bucket: "kronus"
    prefix: "kronus-dev"
    sqliteBackupSchedule: "*/30 * * * *"
    enableSqliteBackupAndSync: true
  applicationCredentials:

twilio:
  key:
  number:
`
