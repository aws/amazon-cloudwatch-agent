import numpy as np
import matplotlib.pyplot as plt

x = np.linspace(-2, 2, 100)
y = np.exp(x)

plt.plot(x, y)
plt.axvline(x=0, color='red', linestyle='--')
plt.axvline(x=1, color='red', linestyle='--')
plt.xlabel('x')
plt.ylabel('e^x')
plt.title('Exponential Function')
plt.grid(True)
plt.savefig('exp_function.png')