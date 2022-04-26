"""
Lifted this from https://github.com/aws/amazon-cloudwatch-agent/issues/447
because I was not able to adequately reproduce the issue natively in Go,
directly in the integration test code.
"""
import json
import logging
import time
from logging.handlers import TimedRotatingFileHandler

# get root logger
logger = logging.getLogger()
logger.setLevel(logging.INFO)

# rotate our log file every 10 seconds
handler = TimedRotatingFileHandler("/tmp/rotate_me.log", when="S", interval=10)
logger.addHandler(handler)

# log a message
logging.info(json.dumps({"Metric": "12345"*10}))
# sleep so that file will rotate upon next log message
time.sleep(15)
# log another message (this one will not appear since byte length of message == byte length of old log file)
logging.info(json.dumps({"Metric": "09876"*10}))
# sleep again so that file will rotate upon next log message
time.sleep(15)
# this message will be partially written
logging.info({"Metric": "1234567890"*10})
