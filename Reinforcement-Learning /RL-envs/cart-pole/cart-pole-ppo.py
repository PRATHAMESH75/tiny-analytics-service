import gym
from stable_baselines3 import PPO
from stable_baselines3.common.vec_env import DummyVecEnv
import numpy as np

# Create and wrap the environment
env = DummyVecEnv([lambda: gym.make("CartPole-v1")])

# Initialize PPO model
model = PPO("MlpPolicy", env, verbose=1)

# Train the model sufficiently to reach score 500
model.learn(total_timesteps=100_000)

# Evaluate the trained agent
episodes = 5
scores = []

for ep in range(episodes):
    obs = env.reset()
    done = [False]
    score = 0
    while not done[0]:
        action, _ = model.predict(obs, deterministic=True)
        obs, reward, done, info = env.step(action)
        score += reward[0]
    scores.append(score)
    print(f"Episode {ep + 1} Score: {score}")

# Summary
print("\n✅ Average Score over", episodes, "episodes:", np.mean(scores))
