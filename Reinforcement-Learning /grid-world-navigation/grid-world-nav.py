import numpy as np
import matplotlib.pyplot as plt
import matplotlib.animation as animation
from matplotlib.colors import ListedColormap
import time

# --- Helper function for Reward Shaping ---
def manhattan_distance(p1, p2):
    return abs(p1[0] - p2[0]) + abs(p1[1] - p2[1])

# --- 1. Environment and Agent Classes (Unchanged) ---

class GridWorld:
    def __init__(self, size=20):
        self.size = size
        self.start_state = (0, 0)
        self.goal_state = (size - 1, size - 1)
        self.obstacles = [(1,0), (1,1), (1,2), (1,3), (1,4), (1,5), (1,6), (1,7), (1,8), (1,9),
                          (3,2), (3,3), (3,4), (3,5), (3,6), (3,7), (3,8), (3,9), (3,10), (3,11),
                          (5,0), (5,1), (5,2), (5,3), (5,4), (5,12), (5,13), (5,14), (5,15), (5,16),
                          (7,6), (7,7), (7,8), (7,9), (7,10), (7,11), (7,12), (7,13), (7,14), (7,15),
                          (9,1), (9,2), (9,3), (9,4), (9,17), (9,18), (9,19),
                          (11,5), (11,6), (11,7), (11,8), (11,9), (11,10), (11,11), (11,12), (11,13),
                          (13,0), (13,1), (13,2), (13,14), (13,15), (13,16), (13,17), (13,18), (13,19),
                          (15,3), (15,4), (15,5), (15,6), (15,7), (15,8), (15,9), (15,10), (15,11), (15,12),
                          (17,1), (17,2), (17,13), (17,14), (17,15), (17,16), (17,17), (17,18),
                          (19,4), (19,5), (19,6), (19,7), (19,8), (19,9), (19,10), (19,11), (19,12), (19,13),
                          (2,15), (4,18), (6,5), (8,17), (10,7), (12,3), (14,9), (16,19), (18,0), (18,8)]
        self.agent_pos = self.start_state
        self.actions = [0, 1, 2, 3] # 0:Up, 1:Down, 2:Left, 3:Right

    def reset(self):
        self.agent_pos = self.start_state
        return self.agent_pos

    def step(self, action):
        current_pos = self.agent_pos
        row, col = current_pos
        if action == 0: row = max(0, row - 1)
        elif action == 1: row = min(self.size - 1, row + 1)
        elif action == 2: col = max(0, col - 1)
        elif action == 3: col = min(self.size - 1, col + 1)
        next_state = (row, col)
        if next_state == self.goal_state:
            reward = 100; done = True
        elif next_state in self.obstacles:
            reward = -10; done = True
        else:
            dist_before = manhattan_distance(current_pos, self.goal_state)
            dist_after = manhattan_distance(next_state, self.goal_state)
            reward = (dist_before - dist_after) * 0.1 - 0.2; done = False
        self.agent_pos = next_state
        return next_state, reward, done

class Agent:
    def __init__(self, env, learning_rate=0.2, discount_factor=0.99, epsilon=1.0, epsilon_decay=0.99995, epsilon_min=0.01):
        self.env = env
        self.lr = learning_rate
        self.gamma = discount_factor
        self.epsilon = epsilon
        self.epsilon_decay = epsilon_decay
        self.epsilon_min = epsilon_min
        self.q_table = np.zeros((env.size, env.size, len(env.actions)))

    def choose_action(self, state, exploit_only=False):
        if not exploit_only and np.random.rand() < self.epsilon:
            return np.random.choice(self.env.actions)
        else:
            return np.argmax(self.q_table[state])

    def update_q_value(self, state, action, reward, next_state):
        old_value = self.q_table[state][action]
        next_max = np.max(self.q_table[next_state])
        new_value = old_value + self.lr * (reward + self.gamma * next_max - old_value)
        self.q_table[state][action] = new_value

    def update_epsilon(self):
        if self.epsilon > self.epsilon_min:
            self.epsilon *= self.epsilon_decay

# --- 2. Visualization Class ---

class GridWorldVisualizer:
    def __init__(self, env, agent):
        self.env = env
        self.agent = agent
        self.fig, self.ax = plt.subplots(figsize=(8, 8))
        self.cmap = ListedColormap(['white', 'black', 'limegreen', 'crimson'])

    def update_plot(self, frame_data):
        """Updates the plot for a single frame of the policy animation."""
        agent_pos, step_num = frame_data
        self.ax.clear()
        grid_data = np.zeros((self.env.size, self.env.size))
        for obs in self.env.obstacles:
            grid_data[obs] = 1
        grid_data[self.env.goal_state] = 2
        grid_data[agent_pos] = 3
        self.ax.imshow(grid_data, cmap=self.cmap, interpolation='nearest')
        self.ax.set_title(f"Learned Policy - Step: {step_num}")
        self.ax.set_xticks([])
        self.ax.set_yticks([])
        return [self.ax]

    def policy_generator(self):
        """Yields the agent's position as it follows the learned policy."""
        state = self.env.reset()
        done = False
        steps = 0
        while not done and steps < 200:
            action = self.agent.choose_action(state, exploit_only=True)
            state, _, done = self.env.step(action)
            steps += 1
            yield state, steps
            
    def animate_policy(self):
        """Runs the animation of the final learned policy."""
        print("Visualizing the learned policy... Close the plot window to exit.")
        ani = animation.FuncAnimation(self.fig, self.update_plot, frames=self.policy_generator,
                                      blit=False, repeat=False, interval=100)
        plt.show()

# --- 3. Training Function ---

def train_agent(env, agent, episodes=75000):
    """Performs a full, non-visual training session."""
    print(f"Starting training for {episodes} episodes... 🤖")
    for episode in range(episodes):
        state = env.reset()
        done = False
        steps = 0
        while not done and steps < 400:
            action = agent.choose_action(state)
            next_state, reward, done = env.step(action)
            agent.update_q_value(state, action, reward, next_state)
            state = next_state
            steps += 1
        agent.update_epsilon()
        if (episode + 1) % 5000 == 0:
            print(f"  ...Episode {episode + 1} completed. Epsilon: {agent.epsilon:.3f}")
    print("Training complete! ✅")
    return agent

# --- 4. Main Execution Block ---

if __name__ == "__main__":
    # Initialize environment and agent
    env = GridWorld(size=20)
    agent = Agent(env)
    
    # Step 1: Train the agent fully without any UI
    trained_agent = train_agent(env, agent, episodes=75000)
    
    # Step 2: Visualize the final, successful policy
    visualizer = GridWorldVisualizer(env, trained_agent)
    visualizer.animate_policy()

    print("\nProgram finished.") 


