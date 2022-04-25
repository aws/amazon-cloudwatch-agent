import json
import logging
import time
from logging.handlers import TimedRotatingFileHandler

# get root logger
logger = logging.getLogger()
logger.setLevel(logging.INFO)

# rotate our log file every 3 minutes
handler = TimedRotatingFileHandler("/tmp/rotate_me.log", when="M", interval=3)
logger.addHandler(handler)

# log a message
logging.info(json.dumps({"Metric": "12345"*10}))
# sleep for 4 minutes so that file will rotate upon next log message
time.sleep(60*4)
# log another message (this one will not appear since byte length of message == byte length of old log file)
logging.info(json.dumps({"Metric": "09876"*10}))
# sleep for another 4 minutes so that file will rotate upon next log message
time.sleep(60*4)
# this message will be partially written
logging.info({"Metric": "1234567890"*10})
